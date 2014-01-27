package main

import (
    "bufio"
    "bytes"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "net"
    "net/http"
    "net/http/httputil"
    "net/textproto"
    "net/url"
    "os"
    "os/exec"
    "strconv"
    "strings"
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

    self.logger.Printf("clip start parsing")
    self.fetchUrl, err = self.parser.Parse()
    if err != nil {
        self.logger.Printf("clip parsing error: %s", err.Error())
        return err
    }

    var conn net.Conn
    var total int64
    var br *bufio.Reader
    redirects := 10
redirects_loop:
    for redirects > 0 {

        req, err := http.NewRequest("GET", self.fetchUrl, nil)
        if err != nil {
            return err
        }

        fetchUrl, err := url.Parse(self.fetchUrl)
        if err != nil {
            return err
        }

        conn, err = net.DialTimeout("tcp", fmt.Sprintf("%s:80", fetchUrl.Host), time.Second*10)
        if err != nil {
            return err
        }

        br = bufio.NewReader(conn)
        tr := textproto.NewReader(br)
        reqDump, err := httputil.DumpRequest(req, false)
        if err != nil {
            return err
        }
        _, err = conn.Write(reqDump)
        if err != nil {
            return err
        }

        s, err := tr.ReadLine()
        if err != nil {
            return err
        }
        self.logger.Printf(s)
        items := strings.SplitN(s, " ", 3)
        if len(items) != 3 || items[0] != "HTTP/1.1" {
            return fmt.Errorf("malformed http response")
        }
        status, err := strconv.Atoi(items[1])
        if err != nil {
            return err
        }
        h, err := tr.ReadMIMEHeader()
        if err != nil {
            return err
        }
        self.logger.Printf("%+v", h)
        switch status {
        case http.StatusOK:
            total, err = strconv.ParseInt(h.Get("Content-Length"), 10, 64)
            if err != nil {
                return err
            }
            break redirects_loop
        case http.StatusFound:
            self.fetchUrl = h.Get("Location")
            redirects--
        default:
            self.logger.Printf("unsupported status %s, %s", items[1], items[2])
            return ErrDownloadIO
        }
    }
    defer conn.Close()
    if redirects == 0 {
        return fmt.Errorf("too many redirects")
    }
    self.total = total
    deadline := time.Now().Add(time.Duration((self.total >> 17)) * time.Second)
    self.logger.Printf("deadline: %s", deadline.String())
    conn.SetReadDeadline(deadline)
    //var total32 int = int(total)
    saved := make([]byte, total, total)
    var nCopy, finished int
    buf, err := br.Peek(br.Buffered())
    if err != nil {
        return err
    }
    nCopy = copy(saved[0:], buf)
    if nCopy != br.Buffered() {
        return ErrDownloadIO
    }
    finished += nCopy
    self.logger.Printf("start downloading: %s", self.fetchUrl)
io_loop:
    for {
        select {
        case <-self.chQuit:
            err = ErrDownloadCancelled
            break io_loop
        case stCh := <-self.chProgress:
            p := Progress{Total: self.total, Finished: int64(finished)}
            stCh <- p
        default:
            nCopy, err = conn.Read(saved[finished:])
            finished += nCopy
            //if finished == total32 {
            //self.logger.Printf("meet eof")
            //err = nil
            //break io_loop
            //}
            //if finished > total32 {
            //err = ErrDownloadIO
            //self.logger.Printf("finished > total")
            //break io_loop
            //}
            if err == io.EOF {
                self.logger.Printf("meet eof2")
                err = nil
                break io_loop
            }
            if err != nil {
                break io_loop
            }
        }
    }
    if err != nil {
        return err
    }

    self.logger.Println("writing data to file")
    err = ioutil.WriteFile(self.output, saved, 0644)
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
