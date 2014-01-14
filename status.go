package main

// cancel a downloading will fallback to StatusUnstarted
const (
    StatusUnstarted int = iota
    StatusWaiting
    StatusDownloading
    StatusCombining
    StatusConverting
    StatusSuccess
    StatusFailure
)
