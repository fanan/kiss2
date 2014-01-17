package main

import (
    "fmt"
    "log"
    "net/url"
    "os"
    "path"
    "strconv"
    "strings"
)

type Video struct {
    nClips     int
    Status     int
    Id         int
    err        error
    Name       string
    Url        string
    chProgress chan chan Progress
    chQuit     chan bool
    chErr      chan error
    parser     VideoParser
    clips      []*Clip
    logger     *log.Logger
}

func (self *Video) Info() *VideoInfo {
    var msg string
    if self.err != nil {
        msg = self.err.Error()
    }
    return &VideoInfo{Status: self.Status, Err: msg, Url: self.Url, Name: self.Name, Id: self.Id}
}

func (self *Video) Parse() error {
    self.logger.Println("start parsing")
    // init parser
    u, err := url.Parse(self.Url)
    if err != nil {
        return err
    }
    if self.parser == nil {
        if strings.Contains(strings.ToLower(u.Host), "sohu") {
            self.parser = new(SohuVideoParser)
        } else if strings.Contains(strings.ToLower(u.Host), "youku") {
            self.parser = new(YoukuVideoParser)
        } else {
            return ErrSiteUnsupported
        }
        self.parser.SetOwner(self)
    }

    self.err = self.parser.Parse()
    if self.err != nil {
        self.Status = StatusFailure
        self.logger.Printf("parse error: %s", self.err.Error())
    } else {
        self.Status = StatusWaiting
    }
    self.logger.Println("finished parsing")
    return self.err
}

func (self *Video) Download() {
    defer func() {
        close(self.chProgress)
        close(self.chQuit)
        self.chErr <- self.err
        close(self.chErr)
    }()
    self.logger.Println("waiting the download queue")
    self.chProgress = make(chan chan Progress)
    self.chQuit = make(chan bool)
    self.chErr = make(chan error)

    // waiting to download ro quit signal
    select {
    case Queue <- 1:
        self.Status = StatusDownloading
        DefaultControlCenter.Save()
        self.logger.Println("start downloading")

    case <-self.chQuit:
        self.err = ErrDownloadCancelled
        self.logger.Println("cancelled while waiting")
        return
    }

    //start downloading
    for idx, clip := range self.clips {
        fn := fmt.Sprintf("%s.part-%d", path.Join(Temp, "kiss_download-"+strconv.Itoa(self.Id)), idx) //clip's filename contain's no '|'
        self.logger.Printf("start downloading: %s, %d/%d", fn, idx, self.nClips)
        clip.SetOutput(fn)
        chFi := make(chan error)
        go clip.Download(chFi)
    clip_download_loop:
        for {
            select {
            case self.err = <-chFi:
                self.logger.Println("received clip finish signal")
                if self.err == nil {
                    self.logger.Println("clip finished downloading")
                    break clip_download_loop
                } else {
                    self.logger.Printf("download failed, error: %s", self.err.Error())
                    return
                }
            case <-self.chQuit:
                self.logger.Println("received quit signal, quiting")
                clip.Cancel()
            case ch := <-self.chProgress:
                var total, finished int64
                for i := 0; i < idx; i++ {
                    total += self.clips[i].total
                    finished += self.clips[i].total
                }
                for i := idx + 1; i < self.nClips; i++ {
                    total += self.clips[i].total
                }
                pg, err := clip.Progress()
                if err != nil {
                    total += clip.total
                    finished += clip.total
                } else {
                    total += pg.Total
                    finished += pg.Finished
                }
                pg.Total, pg.Finished = total, finished
                ch <- pg
            }
        }
    }
    return
}

func (self *Video) Cancel() error {
    var err error
    defer func() {
        e := recover()
        if e != nil {
            err = ErrCannotCancel
        }
    }()
    switch self.Status {
    case StatusWaiting, StatusDownloading:
        self.logger.Println("sending quit signal")
        self.chQuit <- true
    default:
        self.logger.Println("cannot quit")
        return ErrCannotCancel
    }
    return nil
}

func (self *Video) Progress() (ds Progress, err error) {
    defer func() {
        e := recover()
        if e != nil {
            err = ErrChannelClosed
        }
    }()
    if self.Status != StatusDownloading {
        return ds, ErrIsNotDownloading
    }
    ch := make(chan Progress)
    self.chProgress <- ch
    ds = <-ch
    return ds, err
}

func (self *Video) Combine() error {
    output := path.Join(Output, self.Name+".mp4")
    os.Remove(output)
    if self.nClips == 1 {
        self.logger.Println("only one clip, need not combine")
        vr, ar, isMp4, err := detectVideoInfo(self.clips[0].output)
        if err != nil {
            return err
        }
        // just rename
        if isMp4 == true && vr == false && ar == false {
            self.logger.Printf("renaming: %s->%s", self.clips[0].output, output)
            err = os.Rename(self.clips[0].output, output)
            if err != nil {
                self.logger.Printf("rename error: %s", err.Error())
            }
            return err
        }
        self.logger.Println("reencoding")
        err = self.clips[0].ConvertToTs()
        if err != nil {
            return err
        }
        self.logger.Println("converting")
        err = combineTsToMp4(output, self.clips[0].output+".ts")
        if err != nil {
            self.logger.Println("converting error:", err)
        }
        // remove temporary ts file
        self.logger.Println("remove temporary ts file")
        os.Remove(self.clips[0].output + ".ts")
        // remove clip download file
        self.logger.Println("remove downloaded file")
        os.Remove(self.clips[0].output)
        return err
    }

    self.logger.Println("multiple clips, need combining")
    inputs := make([]string, 0, self.nClips)
    for _, clip := range self.clips {
        self.logger.Printf("converting %s to %s", clip.output, clip.output+".ts")
        err := clip.ConvertToTs()
        if err != nil {
            self.logger.Printf("converting error: %s", err.Error())
            return err
        }
        inputs = append(inputs, clip.output+".ts")
    }
    self.logger.Println("combining")
    err := combineTsToMp4(output, inputs...)
    if err != nil {
        self.logger.Printf("combining error: %s", err.Error())
    }
    for _, clip := range self.clips {
        self.logger.Printf("remove downloaded file: %s", clip.output)
        os.Remove(clip.output)
        self.logger.Printf("remove temporary ts file: %s", clip.output+".ts")
        os.Remove(clip.output + ".ts")
    }
    return err
}

func (self *Video) Clean() {
    for idx, _ := range self.clips {
        fn := fmt.Sprintf("%s.part-%d", path.Join(Temp, self.Name), idx)
        self.logger.Printf("clean temp file: %s", fn)
        os.Remove(fn)
    }
    fn := fmt.Sprintf("%s.combined", path.Join(Temp, self.Name))
    os.Remove(fn)
}

func (self *Video) Do() {
    self.logger = log.New(os.Stdout, fmt.Sprintf("[video:%d]", self.Id), log.Lshortfile|log.Lmicroseconds)

    var err error
    self.Parse()

    // update video name, etc
    DefaultControlCenter.Save()

    if self.err != nil {
        self.Status = StatusFailure
        DefaultControlCenter.Save()
        return
    }

    go self.Download()

    self.Status = StatusWaiting
    DefaultControlCenter.Save()

    err = <-self.chErr
    if err != nil {
        if err == ErrDownloadCancelled {
            self.Status = StatusUnstarted
        } else {
            self.Status = StatusFailure
        }
        DefaultControlCenter.Save()
        return
    }

    self.Status = StatusCombining
    DefaultControlCenter.Save()
    err = self.Combine()
    if err != nil {
        self.Status = StatusFailure
        DefaultControlCenter.Save()
        return
    }

    self.Status = StatusSuccess
    DefaultControlCenter.Save()

    self.logger.Println("video hanled")

    return
}
