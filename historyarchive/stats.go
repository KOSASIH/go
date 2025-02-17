package historyarchive

import "sync/atomic"

// golang will auto wrap them back to 0 if they overflow after addition.
type archiveStats struct {
	requests      atomic.Uint32
	fileDownloads atomic.Uint32
	fileUploads   atomic.Uint32
	cacheHits     atomic.Uint32
	cacheBw       atomic.Uint64
	backendName   string
}

type ArchiveStats interface {
	GetRequests() uint32
	GetDownloads() uint32
	GetUploads() uint32
	GetCacheHits() uint32
	GetCacheBandwidth() uint64
	GetBackendName() string
}

func (as *archiveStats) incrementDownloads() {
	as.fileDownloads.Add(1)
	as.incrementRequests()
}

func (as *archiveStats) incrementUploads() {
	as.fileUploads.Add(1)
	as.incrementRequests()
}

func (as *archiveStats) incrementRequests() {
	as.requests.Add(1)
}

func (as *archiveStats) incrementCacheHits() {
	as.cacheHits.Add(1)
}

func (as *archiveStats) incrementCacheBandwidth(bytes int64) {
	as.cacheBw.Add(uint64(bytes))
}

func (as *archiveStats) GetRequests() uint32 {
	return as.requests.Load()
}

func (as *archiveStats) GetDownloads() uint32 {
	return as.fileDownloads.Load()
}

func (as *archiveStats) GetUploads() uint32 {
	return as.fileUploads.Load()
}

func (as *archiveStats) GetBackendName() string {
	return as.backendName
}
func (as *archiveStats) GetCacheHits() uint32 {
	return as.cacheHits.Load()
}
func (as *archiveStats) GetCacheBandwidth() uint64 {
	return as.cacheBw.Load()
}
