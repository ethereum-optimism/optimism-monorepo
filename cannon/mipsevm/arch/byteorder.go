package arch

type ByteOrder interface {
	Word([]byte) Word
	AppendWord([]byte, Word) []byte
	PutWord([]byte, Word)
}
