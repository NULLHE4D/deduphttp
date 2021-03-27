package main

import (
    "fmt"
    "strings"
    "net/url"
)


// ===== string slice helpers =====

func removeByIndex(s []string, i int) []string {
    s[i] = s[len(s)-1]
    return s[:len(s)-1]
}


func removeByValue(s []string, str string) []string {
    for i, v := range s {
        if v == str {
            return removeByIndex(s, i)
        }
    }
    return s
}


func contains(s []string, str string) bool {
    for _, v := range s {
        if v == str {
            return true
        }
    }
    return false
}


func containsAny(s1, s2 []string) bool {
    for _, v := range s2 {
        if contains(s1, v) {
            return true
        }
    }
    return false
}

// ====================

func getHostname(host string) string {
    u, _ := url.Parse(host)
    hostname := strings.Split(u.Host, ":")[0]
    return hostname
}


func getHttpsHosts(hostname string, wSlash bool) []string {
    if wSlash {
        return []string{fmt.Sprintf("https://%s/", hostname), fmt.Sprintf("https://%s:443/", hostname)}
    } else {
        return []string{fmt.Sprintf("https://%s", hostname), fmt.Sprintf("https://%s:443", hostname)}
    }
}
