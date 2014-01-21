package main

import (
    "bytes"
    "io"
    "log"
    "net/http"
    "os"
    "os/exec"
    "time"
)

type Clip struct {
    parser     ClipParser
    info       interface{}
    output     string
    fetchUrl   string
    total      int64
    finished   int64
    chQuit     chan bool
    chProgress chan chan Progress
    logger     *log.Logger
}

func (self *Clip) SetInfo(info interface{}) {
    self.info = info
}

func (self *Clip) GetInfo() interface{} {
    return self.info
}

func (self *Clip) SetOutput(w string) {
    self.output = w
}

func (self *Clip) download() error {
    var err error
    defer func() {
        close(self.chQuit)
        close(self.chProgress)
    }()
    // init channels
    self.chQuit = make(chan bool)
    self.chProgress = make(chan chan Progress)

    buf := make([]byte, 1024*4)
    var resp *http.Response
    self.logger.Printf("clip start parsing")
    self.fetchUrl, err = self.parser.Parse()
    if err != nil {
        self.logger.Printf("clip parsing error: %s", err.Error())
        return err
    }

    self.logger.Printf("clip parsing finished, got url: %s", self.fetchUrl)
    self.logger.Println("clip start downloading")
    resp, err = http.Get(self.fetchUrl)
    if err != nil {
        self.logger.Printf("clip download error:%s", err.Error())
        return err
    }

    self.logger.Printf("clip length adjust: %d->%d", self.total, resp.ContentLength)
    self.total = resp.ContentLength

    defer resp.Body.Close()

    self.logger.Printf("save to file: %s", self.output)
    fp, err := os.Create(self.output)
    if err != nil {
        self.logger.Printf("save error: %s", err.Error())
        return err
    }

    defer fp.Close()

    lastZero := false
    var lastTime time.Time
io_loop:
    for {
        select {
        case <-self.chQuit:
            err = ErrDownloadCancelled
            self.logger.Printf("downloading error: %s", err.Error())
            break io_loop
        case stCh := <-self.chProgress:
            p := Progress{Total: self.total, Finished: self.finished}
            stCh <- p
        default:
            nr, er := resp.Body.Read(buf)
            if nr != 0 {
                lastZero = false
                nw, ew := fp.Write(buf[0:nr])
                if ew != nil {
                    err = ew
                    self.logger.Printf("downloading error: %s", err.Error())
                    return err
                }
                if nw != nr {
                    err = ErrDownloadIO
                    self.logger.Printf("downloading error: %s", err.Error())
                    return err
                }
                self.finished += int64(nr)
            }
            if lastZero {
                if time.Since(lastTime) > time.Minute*3 {
                    self.logger.Printf("received nothing in last 3 minutes, network error?")
                    err = ErrDownloadIO
                    return err
                }
            } else {
                lastZero = true
                lastTime = time.Now()
            }
            if er == io.EOF {
                err = nil
                self.logger.Printf("downloading finished: meet EOF")
                break io_loop
            }
            if er != nil {
                err = er
                self.logger.Printf("downloading error: %s", err.Error())
                break io_loop
            }
        }
    }

    return err
}

func (self *Clip) Download(ch chan error) {
    var err error
    defer func() {
        self.logger.Println("sending finished channel")
        ch <- err
        self.logger.Println("sent finished channel")
        close(ch)
    }()
    for i := 0; i < Lives; i++ {
        err = self.download()
        if err == nil || err == ErrDownloadCancelled {
            break
        }
    }
    return
}

func (self *Clip) Cancel() error {
    var err error
    defer func() {
        ec := recover()
        if ec != nil {
            err = ErrChannelClosed
        }
    }()
    self.chQuit <- true
    return err
}

func (self *Clip) Progress() (st Progress, err error) {
    defer func() {
        ec := recover()
        if ec != nil {
            st.Finished, st.Total = self.finished, self.total
            err = ErrChannelClosed
        }
    }()
    ch := make(chan Progress)
    self.chProgress <- ch
    st = <-ch
    return st, err
}

func (self *Clip) ConvertToTs() error {
    //cmd ffmpeg -i input_filename -vcodec copy/h264 -acodec copy/acc -bsf:v h264_mp4toannexb -f mpegts output_filename
    vr, ar, _, err := detectVideoInfo(self.output)
    if err != nil {
        return err
    }
    var vc, ac string
    if ar {
        ac = FF_CODEC_AAC
    } else {
        ac = FF_CODEC_COPY
    }
    if vr {
        vc = FF_CODEC_H264
    } else {
        vc = FF_CODEC_COPY
    }
    cmd := exec.Command("ffmpeg", "-i", self.output, "-vcodec", vc, "-acodec", ac, "-bsf:v", "h264_mp4toannexb", "-f", "mpegts", self.output+".ts")
    stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
    cmd.Stderr = stderr
    cmd.Stdout = stdout
    err = cmd.Run()
    if err != nil {
        self.logger.Println(stderr.String())
        return err
    }
    return nil
}

func (self *Clip) remove() error {
    return os.Remove(self.output)
}
