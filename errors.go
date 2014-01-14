package main

import (
    "errors"
)

var (
    ErrDownloadCancelled  error = errors.New("download cancelled")
    ErrDownloadIO         error = errors.New("download io error")
    ErrChannelClosed      error = errors.New("download chan closed, is not downloading")
    ErrCannotCancel       error = errors.New("cannot cancel")
    ErrIsNotDownloading   error = errors.New("is not downloading")
    ErrParseBadFormat     error = errors.New("parse error")
    ErrSiteUnsupported    error = errors.New("site not supported")
    ErrProtcolUnsupported error = errors.New("protocol not supported")
    ErrUrlDuplicated      error = errors.New("url duplicated")
    ErrVideoNotFound      error = errors.New("video not found")
    ErrCannotStart        error = errors.New("cannot start")
)
