package goddddocr

import (
	"errors"
	"fmt"
	"sync/atomic"
)

type OCRPool struct {
	model     Model
	workers   []*OCR
	available chan *OCR
	done      chan struct{}
	closed    atomic.Bool
}

func NewOCRPool(config Config, workers int) (*OCRPool, error) {
	if workers <= 0 {
		return nil, fmt.Errorf("workers must be positive")
	}

	pool := &OCRPool{
		workers:   make([]*OCR, 0, workers),
		available: make(chan *OCR, workers),
		done:      make(chan struct{}),
	}
	for idx := 0; idx < workers; idx++ {
		ocr, err := NewOCR(config)
		if err != nil {
			_ = pool.Close()
			return nil, fmt.Errorf("create OCR worker %d/%d: %w", idx+1, workers, err)
		}
		if idx == 0 {
			pool.model = ocr.Model()
		}
		pool.workers = append(pool.workers, ocr)
		pool.available <- ocr
	}
	return pool, nil
}

func (p *OCRPool) Model() Model {
	if p == nil {
		return ""
	}
	return p.model
}

func (p *OCRPool) Size() int {
	if p == nil {
		return 0
	}
	return len(p.workers)
}

func (p *OCRPool) ClassifyBytesDetailed(data []byte, options *ClassifyOptions) (*ClassifyResult, error) {
	if p == nil || len(p.workers) == 0 || p.closed.Load() {
		return nil, fmt.Errorf("OCR engine is closed")
	}

	var ocr *OCR
	select {
	case <-p.done:
		return nil, fmt.Errorf("OCR engine is closed")
	case ocr = <-p.available:
	}
	defer func() {
		select {
		case <-p.done:
		default:
			p.available <- ocr
		}
	}()
	return ocr.ClassifyBytesDetailed(data, options)
}

func (p *OCRPool) Close() error {
	if p == nil {
		return nil
	}
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(p.done)

	var joined error
	for _, worker := range p.workers {
		joined = errors.Join(joined, worker.Close())
	}
	return joined
}
