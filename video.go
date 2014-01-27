package main

import (
    "fmt"
    "io"
    "log"
    "net/url"
    "os"
    "path"
    "strconv"
    "strings"
    "time"
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

func (self *Video) ProgressInfo() *VideoProgress {
    if self.Status != StatusDownloading {
        return nil
    }
    pg, err := self.Progress()
    if err != nil {
        return nil
    }
    return &VideoProgress{Status: self.Status, Err: "", Url: self.Url, Name: self.Name, Id: self.Id, Total: pg.Total, Finished: pg.Finished}
}

func (self *Video) Parse() error {
    self.logger.Println("start parsing")
    // init parser
    u, err := url.Parse(self.Url)
    if err != nil {
        self.err = err
        return err
    }
    if self.parser == nil {
        if strings.Contains(strings.ToLower(u.Host), "sohu") {
            self.parser = new(SohuVideoParser)
        } else if strings.Contains(strings.ToLower(u.Host), "youku") {
            self.parser = new(YoukuVideoParser)
        } else if strings.Contains(strings.ToLower(u.Host), "iqiyi") {
            self.parser = new(IQiYiVideoParser)
        } else {
            self.err = ErrSiteUnsupported
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
    NewsRoom.Pub(self.Info(), TOPIC_WAIT)
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

    defer func() {
        self.logger.Printf("releasing the download queue")
        <-Queue
    }()

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
    var err error
    var w io.Writer
    logFp, err := os.OpenFile(path.Join(Log, fmt.Sprintf("kiss-%d.log", self.Id)), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        w = os.Stdout
    } else {
        w = logFp
        defer logFp.Close()
    }
    self.logger = log.New(w, fmt.Sprintf("[video:%d]", self.Id), log.Lshortfile|log.Lmicroseconds)

    self.Parse()

    // update video name, etc
    DefaultControlCenter.Save()

    if self.err != nil {
        self.Status = StatusFailure
        DefaultControlCenter.Save()
        NewsRoom.Pub(self.Info(), TOPIC_FAIL)
        return
    }

    go self.Download()

    //inject progress service
    chFinished := make(chan bool)
    go func(ch chan bool) {
    monitor_loop:
        for {
            time.Sleep(time.Second)
            select {
            case <-ch:
                self.logger.Printf("monitor goroutine: quit signal got, exiting")
                break monitor_loop
            default:
                NewsRoom.Pub(self.ProgressInfo(), TOPIC_DOWNLOAD)
            }
        }
        self.logger.Printf("monitor goroutine exited")
    }(chFinished)

    self.Status = StatusWaiting
    DefaultControlCenter.Save()

    err = <-self.chErr
    self.logger.Printf("ok, video download finished or an error occurred during downloading")
    self.logger.Printf("sending signal to monitor goroutine")
    chFinished <- true
    self.logger.Printf("monitor goroutine signal sent")
    close(chFinished)
    if err != nil {
        if err == ErrDownloadCancelled {
            self.Status = StatusUnstarted
            NewsRoom.Pub(self.Info(), TOPIC_CANCEL)
        } else {
            self.Status = StatusFailure
            NewsRoom.Pub(self.Info(), TOPIC_FAIL)
        }
        DefaultControlCenter.Save()
        return
    }

    self.Status = StatusCombining
    NewsRoom.Pub(self.Info(), TOPIC_COMBINE)
    DefaultControlCenter.Save()
    err = self.Combine()
    if err != nil {
        self.Status = StatusFailure
        NewsRoom.Pub(self.Info(), TOPIC_FAIL)
        DefaultControlCenter.Save()
        return
    }

    self.Status = StatusSuccess
    NewsRoom.Pub(self.Info(), TOPIC_SUCCESS)
    DefaultControlCenter.Save()

    self.logger.Println("video hanled")

    return
}
