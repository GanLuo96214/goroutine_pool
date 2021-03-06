package pool

import (
	"errors"
	"sync"
	"time"
)

type buildInLoopPool struct {
	*Status

	containerPrepareNext chan *bool

	reviseContainerRunningCountAsExpectCountMutex sync.Mutex

	runFunc func(containerEnd func(), containerIndex uint64)
}

var (
	newBuildInLoopPoolRunFuncIsNil = errors.New("func is nil,the pool would not start")
)

func newBuildInLoopPool(
	expectRunningCount uint64,
	runFunc func(containerEnd func(), containerIndex uint64),
) (p *buildInLoopPool, err error) {

	p = new(buildInLoopPool)
	p.Status = new(Status)

	// set default revise  running count
	err = p.SetDetectExpectDuration(defaultDetectExpectDuration)
	if err != nil {
		return nil, err
	}

	// set expect running  count
	err = p.SetExpectRunningCount(expectRunningCount)
	if err != nil {
		return nil, err
	}

	if runFunc == nil {
		err = newBuildInLoopPoolRunFuncIsNil
		return nil, err
	}

	p.runFunc = runFunc

	p.containerPrepareNext = make(chan *bool)

	// if GetNowRunningCount() < GetExpectRunningCount() then create containers
	go p.reviseContainerRunningCountAsExpectCount()
	// if GetNowRunningCount() > GetExpectRunningCount() then release containers
	go p.reviseOverflowContainer()

	return p, err
}

func (p *buildInLoopPool) containerStart(containerBreaker *bool, containerIndex uint64) {
	p.incrNowRunningCount()
	defer p.decrNowRunningCount()

	p.reviseContainerRunningCountAsExpectCountMutex.Unlock()

	containerEnd := func() {
		*containerBreaker = true
	}

	for *containerBreaker == false {
		p.runFunc(containerEnd, containerIndex)
		p.containerPrepareNext <- containerBreaker
	}

	return
}

func (p *buildInLoopPool) reviseContainerRunningCountAsExpectCount() {
	for {
		p.reviseContainerRunningCountAsExpectCountMutex.Lock()

		if p.GetNowRunningCount() == p.GetExpectRunningCount() || p.GetNowRunningCount() > p.GetExpectRunningCount() {
			p.reviseContainerRunningCountAsExpectCountMutex.Unlock()
			time.Sleep(p.GetDetectExpectDuration())
			continue
		}

		go p.containerStart(p.newContainerBreaker(), p.newContainerIndex())
	}
}

func (p *buildInLoopPool) reviseOverflowContainer() {
	for {

		containerBreaker := <-p.containerPrepareNext

		if p.GetNowRunningCount() > p.GetExpectRunningCount() {
			*containerBreaker = true
		}

	}
}
