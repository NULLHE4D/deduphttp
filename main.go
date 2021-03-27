package main

import (
    "fmt"
    "flag"
    "os"
    "bufio"
)


var config Config

func main() {

    // setup configuration flags
    flag.IntVar(&config.concurrency1, "c1", 10, "concurrency for requests sent with native http library")
    flag.IntVar(&config.concurrency2, "c2", 5, "concurrency for headless browsing")
    flag.BoolVar(&config.filter1, "f1", true, "HTTPS redirect filter")
    flag.BoolVar(&config.filter2, "f2", true, "common host redirect filter")
    flag.IntVar(&config.delay, "d", 10, "delay (in seconds) before determining finalPage for common host redirect filter")

    flag.Parse()

    var hosts []string

    // read hosts from stdin
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        hosts = append(hosts, scanner.Text())
    }

    // apply filters if filter flags are set
    if config.filter1 {
        hosts = HttpsRedirectFilter(hosts)
    }

    if config.filter2 {
        hosts = CommonHostRedirectFilter(hosts)
    }

    // output filtered hosts to stdout
    for _, v := range hosts {
        fmt.Println(v)
    }

}
