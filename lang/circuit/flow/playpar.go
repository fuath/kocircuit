//
// Copyright © 2018 Aljabr, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package flow

import "github.com/kocircuit/kocircuit/lang/circuit/model"

type stepResult struct {
	Step   *model.Step
	Result Edge
	Error  error
	Panic  interface{}
}

func playPar(f *model.Func, StepPlayer StepPlayer) (r map[*model.Step]Edge, err error) {
	cross := map[*model.Step]chan Edge{}
	for _, s := range f.Step {
		cross[s] = make(chan Edge, len(f.Spread[s]))
	}
	done := make(chan *stepResult, len(f.Step))
	abort := make(chan bool)
	for i := 0; i < len(f.Step); i++ {
		s := f.Step[len(f.Step)-1-i] // iterate steps in forward time order
		// start step co-routine
		go func() {
			// wait for each incoming edge to pass a value, or an abort signal
			gather := make([]GatherEdge, len(s.Gather))
			for j, g := range s.Gather {
				select {
				case <-abort:
					return
				case edge := <-cross[g.Step]:
					gather[j] = GatherEdge{Field: g.Field, Edge: edge}
				}
			}
			// catch a panic from the step execution
			defer func() {
				if r := recover(); r != nil {
					done <- &stepResult{Step: s, Panic: r}
				}
			}()
			if sReturns, err := StepPlayer.PlayStep(s, gather); err != nil {
				done <- &stepResult{Step: s, Error: err}
			} else {
				for j := 0; j < len(f.Spread[s]); j++ {
					cross[s] <- sReturns
				}
				done <- &stepResult{Step: s, Result: sReturns}
			}
		}()
	}
	r = map[*model.Step]Edge{}
	aborting := false
	var stepPanic interface{}
	for i := 0; !aborting && i < len(f.Step); i++ {
		select {
		case sr := <-done:
			if sr.Panic != nil {
				stepPanic = sr.Panic
				close(abort)
				aborting = true
			} else if sr.Error != nil {
				err = sr.Error
				close(abort)
				aborting = true
			} else {
				r[sr.Step] = sr.Result
			}
		}
	}
	if stepPanic != nil {
		panic(stepPanic)
	}
	if err != nil {
		return nil, err
	}
	return
}
