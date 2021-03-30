package main

import (
    "fmt"
    "context"
    "time"
    "sync"
    "os"
    "io"
    "io/ioutil"
    "net/http"
    "crypto/tls"

    "github.com/chromedp/chromedp"
    "github.com/chromedp/cdproto/page"
)


const userAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:85.0) Gecko/20100101 Firefox/85.0"

var tr *http.Transport = &http.Transport{
    MaxIdleConns: 30,
    MaxIdleConnsPerHost: -1,
    IdleConnTimeout: time.Second,
    DisableKeepAlives: true,
    TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

// todo: use different redirect configuration
var client *http.Client = &http.Client {
    Transport: tr,
    Timeout: time.Second * 10,
}

var opts []chromedp.ExecAllocatorOption = append(chromedp.DefaultExecAllocatorOptions[:],
    //chromedp.Flag("headless", false),
    chromedp.Flag("ignore-certificate-errors", true),
)


// group hosts based on hostname
func GroupHosts(hosts []string) map[string][]string {

    grouped := make(map[string][]string)
    tmp := make([]string, len(hosts))
    copy(tmp, hosts)

    for len(tmp) > 0 {

        var group []string
        hostname1 := getHostname(tmp[0])

        i := 0
        for i < len(tmp) {
            if hostname2 := getHostname(tmp[i]); hostname2 == hostname1 {
                group = append(group, tmp[i])
                tmp = removeByIndex(tmp, i)
            } else {
                i++
            }
        }

        grouped[hostname1] = group

    }

    return grouped

}


func HttpsRedirectFilter(hosts []string) []string {

    groupedHosts := GroupHosts(hosts)
    hostsChan := make(chan string)
    var errHosts []string
    var hostsToFilter []string
    var wg sync.WaitGroup
    var m sync.Mutex

    // after we group the hosts based on hostname we check which groups/hosts are applicable for this filter
    // by checking if the group contains standard HTTPS host(s) (https://whatever.com or https://whatever.com:443)
    for hostname, group := range groupedHosts {

        if !containsAny(group, getHttpsHosts(hostname, false)) {
            continue
        } else {
            if len(group) > 1 {
                for _, host := range group {
                    hostsToFilter = append(hostsToFilter, host)
                }
                hostsToFilter = removeByValue(hostsToFilter, fmt.Sprintf("https://%s", hostname))
                hostsToFilter = removeByValue(hostsToFilter, fmt.Sprintf("https://%s:443", hostname))
            }
        }

    }

    // we then send requests to hosts in hostsToFilter, check their history to
    // identify any redirects to their standard HTTPS host counterparts, and
    // remove them from the main hosts slice
    for i := 0; i < config.concurrency1; i++ {

        wg.Add(1)
        go func(wg *sync.WaitGroup, m *sync.Mutex, hosts, errHosts *[]string, hostsChan <-chan string) {

            for u := range hostsChan {

                req, err := http.NewRequest("GET", u, nil)
                if err != nil {
                    fmt.Fprintf(os.Stderr, "%v: %v\n", u, err)
                    *errHosts = append(*errHosts, u)
                    continue
                }

                req.Header.Set("User-Agent", userAgent)
                req.Header.Set("Connection", "Close")
                req.Close = true

                res, err := client.Do(req)
                if res != nil {
                    io.Copy(ioutil.Discard, res.Body)
                    res.Body.Close()
                }

                if err != nil {
                    fmt.Fprintf(os.Stderr, "%v: %v\n", u, err)
                    *errHosts = append(*errHosts, u)
                    continue
                }

                // get redirection history
                var history []string
                for res != nil {
                    req := res.Request
                    history = append(history, req.URL.String())
                    res = req.Response
                }

                if containsAny(history, getHttpsHosts(getHostname(u), true)) {
                    m.Lock()
                    *hosts = removeByValue(*hosts, u)
                    m.Unlock()
                }

            }

            wg.Done()

        }(&wg, &m, &hosts, &errHosts, hostsChan)

    }

    for _, v := range hostsToFilter {
        hostsChan <- v
    }

    close(hostsChan)
    wg.Wait()

    for _, v := range errHosts {
        hosts = removeByValue(hosts, v)
    }

    return hosts

}


// GenHostData does the headless browsing and populates HostData structs according
// to the dataOpts parameter. it was made this way to avoid rewriting several
// functions with the same first 20-ish lines of code for populating different
// fields of the HostData struct
func GenHostData(allocCtx context.Context, hd *HostData, dataOpts []bool) {

    ctx, cancel := chromedp.NewContext(allocCtx)
    defer cancel()

    // handle javascript dialogs like alert()
    chromedp.ListenTarget(ctx, func(ev interface{}) {
        if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
            go func() {
                if err := chromedp.Run(ctx,
                    page.HandleJavaScriptDialog(true),
                ); err != nil {
                    fmt.Fprintf(os.Stderr, "%v: %v\n", hd.host, err)
                    return
                }
            }()
        }
    })

    if dataOpts[0] { // get finalUrl

        var jsOut string
        if err := chromedp.Run(ctx,
            chromedp.Navigate(hd.host),
            chromedp.Sleep(time.Duration(config.delay) * time.Second),
            chromedp.Evaluate("window.location.href;", &jsOut),
        ); err != nil {
            fmt.Fprintf(os.Stderr, "%v: %v\n", hd.host, err)
            return
        }

        hd.finalUrl = jsOut

    }

}


func CommonHostRedirectFilter(hosts []string) []string {

    var wg sync.WaitGroup
    var hdSlice []*HostData
    hdChan := make(chan *HostData)

    allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
    defer cancel()

    // get finalPage of hosts using headless browsing
    for i := 0; i < config.concurrency2; i++ {

        wg.Add(1)
        go func(wg *sync.WaitGroup, allocCtx context.Context) {

            for hd := range hdChan {
                GenHostData(allocCtx, hd, []bool{true})
            }

            wg.Done()

        }(&wg, allocCtx)

    }

    for _, v := range hosts {
        hdSlice = append(hdSlice, &HostData{host: v})
        hdChan <- hdSlice[len(hdSlice)-1]
    }

    close(hdChan)
    wg.Wait()

    // only keep one host for every unique finalPage, prioritizing a host
    // with the same hostname as the finalPage URL
    filteredMap := make(map[string]string)
    for _, hd := range hdSlice {
        if _, ok := filteredMap[hd.finalUrl]; (!ok || getHostname(hd.finalUrl) == getHostname(hd.host)) && len(hd.finalUrl) > 0 {
            filteredMap[hd.finalUrl] = hd.host
        }
    }

    hosts = []string{}
    for _, v := range filteredMap {
        hosts = append(hosts, v)
    }

    return hosts

}
