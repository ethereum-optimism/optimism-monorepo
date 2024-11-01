package srcmap

import (
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

type LineCol struct {
	Line uint32
	Col  uint32
}

type InstrMapping struct {
	S int32 // start offset in bytes within source (negative when non-existent!)
	L int32 // length in bytes within source (negative when non-existent!)
	F int32 // file index of source (negative when non-existent!)
	J byte  // jump type (i=into, o=out, -=regular)
	M int32 // modifier depth
}

func parseInstrMapping(last InstrMapping, v string) (InstrMapping, error) {
	data := strings.Split(v, ":")
	out := last
	if len(data) < 1 {
		return out, nil
	}
	if len(data) > 5 {
		return out, fmt.Errorf("unexpected length: %d", len(data))
	}
	var err error
	parse := func(x string) int32 {
		p, e := strconv.ParseInt(x, 10, 32)
		err = e
		return int32(p)
	}
	if data[0] != "" {
		out.S = parse(data[0])
	}
	if len(data) < 2 || err != nil {
		return out, err
	}
	if data[1] != "" {
		out.L = parse(data[1])
	}
	if len(data) < 3 || err != nil {
		return out, err
	}
	if data[2] != "" {
		out.F = parse(data[2])
	}
	if len(data) < 4 || err != nil {
		return out, err
	}
	if data[3] != "" {
		out.J = data[3][0]
	}
	if len(data) < 5 || err != nil {
		return out, err
	}
	if data[4] != "" {
		out.M = parse(data[4])
	}
	return out, err
}

func loadLineColData(srcFs fs.FS, srcPath string) ([]LineCol, error) {
	dat, err := fs.ReadFile(srcFs, srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source %q: %w", srcPath, err)
	}
	datStr := string(dat)
	out := make([]LineCol, len(datStr))
	line := uint32(1)
	lastLinePos := uint32(0)
	for i, b := range datStr { // iterate the utf8 or the bytes?
		col := uint32(i) - lastLinePos
		out[i] = LineCol{Line: line, Col: col}
		if b == '\n' {
			lastLinePos = uint32(i)
			line += 1
		}
	}
	return out, nil
}

type SourceID uint64

func (id *SourceID) UnmarshalText(data []byte) error {
	v, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil {
		return err
	}
	*id = SourceID(v)
	return nil
}

// SourceMap is a util to map solidity deployed-bytecode positions
// to source-file, line and column position data.
// It is best used in combination with foundry.SourceMapFS to load the source-map.
// The source-map functionality is tested as part of the FS.
type SourceMap struct {
	srcFs       fs.FS
	srcIDToPath map[SourceID]string
	// per source, source offset -> line/col
	// This data is lazy-loaded.
	PosData map[SourceID][]LineCol
	// per bytecode byte, byte index -> instr
	Instr []InstrMapping
}

// Info translates a program-counter (execution position in the EVM bytecode)
// into the source-code location that is being executed.
// This location is the source file-path, the line number, and column number.
// This may return an error, as the source-file is lazy-loaded to calculate the position data.
func (s *SourceMap) Info(pc uint64) (source string, line uint32, col uint32, err error) {
	if pc >= uint64(len(s.Instr)) {
		source = "invalid"
		return
	}
	instr := s.Instr[pc]
	if instr.F < 0 || instr == (InstrMapping{}) {
		return "generated", 0, 0, nil
	}
	id := SourceID(instr.F)
	if _, ok := s.srcIDToPath[id]; !ok {
		source = "unknown"
		return
	}
	source = s.srcIDToPath[id]
	if instr.S < 0 {
		return
	}
	posData, ok := s.PosData[id]
	if !ok {
		data, loadErr := loadLineColData(s.srcFs, source)
		if loadErr != nil {
			return source, 0, 0, loadErr
		}
		s.PosData[id] = data
		posData = data
	}
	if int(instr.S) >= len(posData) { // possibly invalid / truncated source mapping
		return
	}
	lc := posData[instr.S]
	line = lc.Line
	col = lc.Col
	return
}

// FormattedInfo is a convenience method to run Info, and turn it into a formatted string.
// Any error is turned into a string also, to make this simple to plug into logging.
func (s *SourceMap) FormattedInfo(pc uint64) string {
	f, l, c, err := s.Info(pc)
	if err != nil {
		return "srcmap err:" + err.Error()
	}
	return fmt.Sprintf("%s:%d:%d", f, l, c)
}

// ParseSourceMap parses a solidity sourcemap: mapping bytecode indices to source references.
// See https://docs.soliditylang.org/en/latest/internals/source_mappings.html
//
// The srcIDToPath is the mapping of source files, which will be read from the filesystem
// to transform token numbers into line/column numbers. Source-files are lazy-loaded when needed.
//
// The source identifier mapping can be loaded through a foundry.SourceMapFS,
// also including a convenience util to load a source-map from an artifact.
func ParseSourceMap(srcFs fs.FS, srcIDToPath map[SourceID]string, bytecode []byte, sourceMap string) (*SourceMap, error) {
	instructions := strings.Split(sourceMap, ";")

	srcMap := &SourceMap{
		srcFs:       srcFs,
		srcIDToPath: srcIDToPath,
		PosData:     make(map[SourceID][]LineCol),
		Instr:       make([]InstrMapping, 0, len(bytecode)),
	}

	instIndex := 0

	// bytecode offset to instruction
	lastInstr := InstrMapping{}
	for i := 0; i < len(bytecode); {
		inst := bytecode[i]
		instLen := 1
		if inst >= 0x60 && inst <= 0x7f { // push instructions
			pushDataLen := inst - 0x60 + 1
			instLen += int(pushDataLen)
		}

		var instMapping string
		if instIndex >= len(instructions) {
			// truncated source-map? Or some instruction that's longer than we accounted for?
			// probably the contract-metadata bytes that are not accounted for in source map
		} else {
			instMapping = instructions[instIndex]
		}
		// the last instruction is used to de-dup data with in the source-map encoding.
		m, err := parseInstrMapping(lastInstr, instMapping)
		if err != nil {
			return nil, fmt.Errorf("failed to parse instr element in source map: %w", err)
		}

		for j := 0; j < instLen; j++ {
			srcMap.Instr = append(srcMap.Instr, m)
		}
		lastInstr = m
		i += instLen
		instIndex += 1
	}
	return srcMap, nil
}

func NewSourceMapTracer(srcMaps map[common.Address]*SourceMap, out io.Writer) *SourceMapTracer {
	return &SourceMapTracer{srcMaps, out}
}

type SourceMapTracer struct {
	srcMaps map[common.Address]*SourceMap
	out     io.Writer
}

func (s *SourceMapTracer) info(codeAddr common.Address, pc uint64) string {
	info := "unknown-contract"
	srcMap, ok := s.srcMaps[codeAddr]
	if ok {
		info = srcMap.FormattedInfo(pc)
	}
	return info
}

func (s *SourceMapTracer) OnOpCode(pc uint64, opcode byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
	op := vm.OpCode(opcode)
	var top []string
	stk := scope.StackData()
	for i := len(stk) - 1; i >= 0; i-- {
		top = append(top, stk[i].Hex())
		if len(top) == 4 {
			break
		}
	}
	stkInfo := fmt.Sprintf("[%s]", strings.Join(top, ", "))

	if op.IsPush() {
		var val []byte
		sc, ok := scope.(*vm.ScopeContext)
		if ok {
			start := pc + 1
			end := uint64(op) - uint64(vm.PUSH1) + 1
			val = sc.Contract.Code[start : start+end]
		} else {
			val = []byte("N/A")
		}
		fmt.Fprintf(s.out, "%-40s : pc %x opcode %s (%x) \t| stk[:%d] %s\n", s.info(scope.Address(), pc), pc, op.String(), val, len(top), stkInfo)
		return
	}
	fmt.Fprintf(s.out, "%-40s : pc %x opcode %s \t\t| stk[:%d] %s\n", s.info(scope.Address(), pc), pc, op.String(), len(top), stkInfo)
}

func (s *SourceMapTracer) OnFault(pc uint64, opcode byte, gas, cost uint64, scope tracing.OpContext, depth int, err error) {
	op := vm.OpCode(opcode)
	fmt.Fprintf(s.out, "%-40s: pc %x opcode %s FAULT %v\n", s.info(scope.Address(), pc), pc, op.String(), err)
	fmt.Println("----")
	fmt.Fprintf(s.out, "calldata: %x\n", scope.CallInput())
	fmt.Println("----")
	fmt.Fprintf(s.out, "memory: %x\n", scope.MemoryData())
	fmt.Println("----")
	fmt.Fprintf(s.out, "stack:\n")
	stack := scope.StackData()
	for i := range stack {
		fmt.Fprintf(s.out, "%3d: %x\n", -i, stack[len(stack)-1-i].Bytes32())
	}
}

func (s *SourceMapTracer) Hooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnOpcode: s.OnOpCode,
		OnFault:  s.OnFault,
	}
}
