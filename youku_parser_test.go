package main

import (
    "testing"
)

func TestYouku(t *testing.T) {
    youkuUrl := "http://v.youku.com/v_show/id_XNjU5Mjc0MjAw.html"
    v := new(Video)
    v.Url = youkuUrl
    vp := new(YoukuVideoParser)
    vp.SetOwner(v)
    err := vp.parseVid()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
    err = vp.parseBasicInfo()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
    err = vp.parseVideoTypes()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
    err = vp.parseClips()
    if err != nil {
        t.Fatal(err)
        t.FailNow()
    }
}

func TestGenFileIdMixString(t *testing.T) {
    seed := 80
    expectd := []byte{'W', 'H', 'I', 'O', 'h', 'g', 'E', 'f', 'S', 'M', 'c', 'r', '4', 'w', 'm', 's', 'e', 'B', 'q', '-', '1', 'n', 'i', 't', 'A', 'Y', 'u', 'd', 'p', 'G', 'Q', '\\', 'U', 'x', 'D', '_', 'o', '/', 'V', 'P', 'k', '2', '3', 'X', 'j', '7', ':', 'F', 'y', 'a', '6', 'L', 'z', 'Z', 'b', 'l', 'C', '8', '5', '0', 'T', '.', 'R', 'J', 'v', '9', 'N', 'K'}
    m := getFileIDMixString(seed)
    for i := len(expectd) - 1; i >= 0; i-- {
        if expectd[i] != m[i] {
            t.Fatal("not equal")
            t.FailNow()
        }
    }
}
