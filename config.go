package main

import (
    "encoding/json"
    "errors"
    "io/ioutil"
    "os"
)

var (
    Temp        string = "/tmp"
    Output      string = "/Users/fatman/Movies"
    Db          string = "db.json"
    Log         string = "logs"
    Concurrency int    = 5
    Lives       int    = 5
)

type Config struct {
    Concurrency int    `json:"concurrency"`
    Lives       int    `json:"lives"`
    Temp        string `json:"temp"`
    Output      string `json:"output"`
    Db          string `json:"db"`
    Log         string `json:"log"`
}

func initConfig(fn string) error {
    c, err := ioutil.ReadFile(fn)
    if err != nil {
        return err
    }
    var config Config
    err = json.Unmarshal(c, &config)
    if err != nil {
        return err
    }
    if config.Concurrency <= 0 {
        config.Concurrency = 3
    }
    Concurrency = config.Concurrency
    Queue = make(chan int, Concurrency)

    if config.Lives <= 0 {
        Lives = 3
    } else {
        Lives = config.Lives
    }

    Db = config.Db
    DefaultControlCenter.db = Db
    err = DefaultControlCenter.Init()
    if err != nil {
        if !os.IsNotExist(err) {
            return err
        }
    }

    Temp = config.Temp
    fd, err := os.Open(Temp)
    if err != nil {
        return err
    }
    defer fd.Close()
    st, err := fd.Stat()
    if err != nil {
        return err
    }
    if !st.IsDir() {
        return errors.New("temp is not a directory")
    }

    Output = config.Output
    fo, err := os.Open(Output)
    if err != nil {
        return err
    }
    defer fo.Close()
    so, err := fo.Stat()
    if err != nil {
        return err
    }
    if !so.IsDir() {
        return errors.New("output is not a directory")
    }

    Log = config.Log
    if Log == "" {
        Log = "logs"
    }
    fl, err := os.Open(Log)
    if err != nil {
        return err
    }
    defer fo.Close()
    sl, err := fl.Stat()
    if err != nil {
        return err
    }
    if !sl.IsDir() {
        return errors.New("log is not a directory")
    }

    return nil
}
