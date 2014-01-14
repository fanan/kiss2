package main

import (
    "bytes"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "path"
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
    if self.parser == nil {
        // TODO: add youku and sohu
        self.parser = new(SohuVideoParser)
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
        fn := fmt.Sprintf("%s.part-%d", path.Join(Temp, self.Name), idx)
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
    self.logger.Println("start combining")
    var err error
    out := fmt.Sprintf("%s.combined.mp4", path.Join(Temp, self.Name))
    os.Remove(out)
    if self.nClips != 1 {
        fp, err := ioutil.TempFile(os.TempDir(), "ffmpeg-")
        if err != nil {
            self.err = err
            return err
        }
        defer os.Remove(fp.Name())
        s := ""
        for idx, _ := range self.clips {
            s += fmt.Sprintf("file '%s.part-%d'\n", path.Join(Temp, self.Name), idx)
        }
        _, err = fp.WriteString(s)
        if err != nil {
            self.err = err
            return err
        }
        fp.Close()
        cmd := exec.Command("ffmpeg", "-f", "concat", "-i", fp.Name(), "-c", "copy", out)
        stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
        cmd.Stdout, cmd.Stderr = stdout, stderr
        err = cmd.Run()
        if err != nil {
            self.err = err
            self.logger.Printf("stdout:%s", stdout.String())
            self.logger.Printf("stderr:%s", stderr.String())
            self.logger.Printf("combining error: %s", err.Error())
            return err
        }
    } else {
        self.logger.Println("only one clip, need not combine, just rename")
        err = os.Rename(fmt.Sprintf("%s.part-0", path.Join(Temp, self.Name)), out)
        if err != nil {
            self.err = err
            self.logger.Println("rename error: %s", self.err.Error())
            return err
        }
    }
    self.logger.Println("combine finished")
    return nil
}

func (self *Video) Convert() error {
    self.logger.Println("convert has not implemented")
    return nil
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
    // TODO add clean func after convert implemented

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

    self.Status = StatusConverting
    DefaultControlCenter.Save()
    err = self.Convert()
    if err != nil {
        self.Status = StatusFailure
        DefaultControlCenter.Save()
        return
    }

    self.Status = StatusSuccess
    DefaultControlCenter.Save()

    return
}
