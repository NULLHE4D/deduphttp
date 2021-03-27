package main


// Config is where configuration options
// are stored and retrieved
type Config struct {
    concurrency1    int
    concurrency2    int
    filter1         bool
    filter2         bool
    delay           int
}


// HostData is used to house data when performing headless
// browsing for many hosts before analyzing them
type HostData struct {
    host        string
    ipAddresses []string
    finalUrl  string
    jsSources   []string
    jsHash      string
}
