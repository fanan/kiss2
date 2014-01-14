package main

import (
    "io"
    "log"
    "net/http"
    "os"
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

func (c *Clip) SetInfo(info interface{}) {
    c.info = info
}

func (c *Clip) GetInfo() interface{} {
    return c.info
}

func (c *Clip) SetOutput(w string) {
    c.output = w
}

func (c *Clip) download() error {
    var err error
    defer func() {
        close(c.chQuit)
        close(c.chProgress)
    }()
    // init channels
    c.chQuit = make(chan bool)
    c.chProgress = make(chan chan Progress)

    buf := make([]byte, 1024*4)
    var resp *http.Response
    c.logger.Printf("clip start parsing")
    c.fetchUrl, err = c.parser.Parse()
    if err != nil {
        c.logger.Printf("clip parsing error: %s", err.Error())
        return err
    }

    c.logger.Printf("clip parsing finished, got url: %s", c.fetchUrl)
    c.logger.Println("clip start downloading")
    resp, err = http.Get(c.fetchUrl)
    if err != nil {
        c.logger.Printf("clip download error:%s", err.Error())
        return err
    }

    c.total = resp.ContentLength

    defer resp.Body.Close()

    c.logger.Printf("save to file: %s", c.output)
    fp, err := os.Create(c.output)
    if err != nil {
        c.logger.Printf("save error: %s", err.Error())
        return err
    }

    defer fp.Close()

io_loop:
    for {
        select {
        case <-c.chQuit:
            err = ErrDownloadCancelled
            c.logger.Printf("downloading error: %s", err.Error())
            break io_loop
        case stCh := <-c.chProgress:
            p := Progress{Total: c.total, Finished: c.finished}
            stCh <- p
        default:
            nr, er := resp.Body.Read(buf)
            if nr != 0 {
                nw, ew := fp.Write(buf[0:nr])
                if ew != nil {
                    err = ew
                    c.logger.Printf("downloading error: %s", err.Error())
                    return err
                }
                if nw != nr {
                    err = ErrDownloadIO
                    c.logger.Printf("downloading error: %s", err.Error())
                    return err
                }
                c.finished += int64(nr)
            }
            if er == io.EOF {
                err = nil
                c.logger.Printf("downloading finished: meet EOF")
                break io_loop
            }
            if er != nil {
                err = er
                c.logger.Printf("downloading error: %s", err.Error())
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

func (c *Clip) Cancel() error {
    var err error
    defer func() {
        ec := recover()
        if ec != nil {
            err = ErrChannelClosed
        }
    }()
    c.chQuit <- true
    return err
}

func (c *Clip) Progress() (st Progress, err error) {
    defer func() {
        ec := recover()
        if ec != nil {
            st.Finished, st.Total = c.finished, c.total
            err = ErrChannelClosed
        }
    }()
    ch := make(chan Progress)
    c.chProgress <- ch
    st = <-ch
    return st, err
}
