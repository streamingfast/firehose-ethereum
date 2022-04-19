package types

func NormalizeSignaturePoint(value []byte) []byte {
	if len(value) == 0 {
		return value
	}

	if len(value) < 32 {
		offset := 32 - len(value)

		out := make([]byte, 32)
		copy(out[offset:32], value)

		return out
	}

	return value[0:32]
}
