package main

import (
    "log"
    "os"
    "testing"
)

var pageUrl string = "http://www.iqiyi.com/v_19rrgzy5ls.html"
var testTVid string = "221865100"
var testVid string = "0cebe6abfb8bda607f51aa48c66dfb2a"

func TestIQiYi(t *testing.T) {
    video := new(Video)
    video.Url = pageUrl
    video.logger = log.New(os.Stdout, "[test:]", log.Lshortfile|log.Ltime)
    vp := new(IQiYiVideoParser)
    vp.SetOwner(video)
    // parse vid and tvid
    err := vp.parseVidAndTVid()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
    if vp.vid != testVid || vp.tvid != testTVid {
        t.Fatal("parse result not equal")
        t.FailNow()
    }
    // parse name and aid
    err = vp.parseName()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
    // parse clips info
    err = vp.parseInfo()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
    //
    err = vp.parseClips()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
    cl := video.clips[0]
    s, err := cl.parser.Parse()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
    println(s)
}
