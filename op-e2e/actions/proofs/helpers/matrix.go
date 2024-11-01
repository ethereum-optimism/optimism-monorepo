package helpers

import (
	"fmt"
	"testing"
)

type RunTest[cfg any] func(t *testing.T, testCfg *TestCfg[cfg])

type TestCfg[cfg any] struct {
	Hardfork    *Hardfork
	CheckResult CheckResult
	InputParams []FixtureInputParam
	Custom      cfg
}

type TestCase[cfg any] struct {
	Name        string
	Cfg         cfg
	ForkMatrix  ForkMatrix
	RunTest     RunTest[cfg]
	InputParams []FixtureInputParam
	CheckResult CheckResult
}

type TestMatrix[cfg any] struct {
	CommonInputParams []FixtureInputParam
	TestCases         []TestCase[cfg]
}

func (suite *TestMatrix[cfg]) Run(t *testing.T) {
	for _, tc := range suite.TestCases {
		for _, fork := range tc.ForkMatrix {
			t.Run(fmt.Sprintf("%s-%s", tc.Name, fork.Name), func(t *testing.T) {
				testCfg := &TestCfg[cfg]{
					Hardfork:    fork,
					CheckResult: tc.CheckResult,
					InputParams: append(suite.CommonInputParams, tc.InputParams...),
					Custom:      tc.Cfg,
				}
				tc.RunTest(t, testCfg)
			})
		}
	}
}

func NewMatrix[cfg any]() *TestMatrix[cfg] {
	return &TestMatrix[cfg]{}
}

func (ts *TestMatrix[cfg]) WithCommonInputParams(params ...FixtureInputParam) *TestMatrix[cfg] {
	ts.CommonInputParams = params
	return ts
}

func (ts *TestMatrix[cfg]) AddTestCase(
	name string,
	testCfg cfg,
	forkMatrix ForkMatrix,
	runTest RunTest[cfg],
	checkResult CheckResult,
	inputParams ...FixtureInputParam,
) *TestMatrix[cfg] {
	ts.TestCases = append(ts.TestCases, TestCase[cfg]{
		Name:        name,
		Cfg:         testCfg,
		ForkMatrix:  forkMatrix,
		RunTest:     runTest,
		InputParams: inputParams,
		CheckResult: checkResult,
	})
	return ts
}

type Hardfork struct {
	Name       string
	Precedence int
}

type ForkMatrix = []*Hardfork

// Hardfork definitions
var (
	Regolith = &Hardfork{Name: "Regolith", Precedence: 1}
	Canyon   = &Hardfork{Name: "Canyon", Precedence: 2}
	Delta    = &Hardfork{Name: "Delta", Precedence: 3}
	Ecotone  = &Hardfork{Name: "Ecotone", Precedence: 4}
	Fjord    = &Hardfork{Name: "Fjord", Precedence: 5}
	Granite  = &Hardfork{Name: "Granite", Precedence: 6}
	Holocene = &Hardfork{Name: "Holocene", Precedence: 7}
)

var (
	Hardforks      = ForkMatrix{Regolith, Canyon, Delta, Ecotone, Fjord, Granite, Holocene}
	LatestFork     = Hardforks[len(Hardforks)-1]
	LatestForkOnly = ForkMatrix{LatestFork}
)

func NewForkMatrix(forks ...*Hardfork) ForkMatrix {
	return append(ForkMatrix{}, forks...)
}
