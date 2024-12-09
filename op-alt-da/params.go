package altda

// MaxInputSize ensures the canonical chain cannot include input batches too large to
// challenge in the Data Availability Challenge contract. Value in number of bytes.
// This value can only be changed in a hard fork.
const MaxInputSize = 130672
