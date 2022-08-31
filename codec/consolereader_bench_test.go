// Copyright 2021 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

func BenchmarkConsoleReader(b *testing.B) {
	expectedBlockCount := 33
	data, err := os.ReadFile("testdata/deep-mind.dmlog")
	if err != nil {
		b.Fatal(err)
	}
	lines := bytes.Split(data, []byte("\n"))

	// We don't want to benchmark how fast we can read lines from the file because in the real case,
	// lines are fed by the instrumented process which can be comes in burst or slowly, so we do not
	// care here. We are aiming to improve the console reader main loop itself of consuming lines as
	// fast as possible to transform it.
	readers := make([]*ConsoleReader, b.N)
	for n := 0; n < b.N; n++ {
		channel := make(chan string, len(lines))

		for _, line := range lines {
			channel <- string(line)
		}

		readers[n] = testReaderConsoleReader(b.Helper, channel, func() {})

		// We close it right now, it will still be fully consumed
		close(channel)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		reader := readers[n]

		count := 0
		for {
			_, err := reader.ReadBlock()
			if err == io.EOF {
				break
			}

			if err != nil {
				b.Fatal(err)
			}

			count++
		}

		if count != expectedBlockCount {
			b.Fatal(fmt.Errorf("expected to have read %d blocks but got %d", expectedBlockCount, count))
		}
	}

	b.ReportMetric(float64(expectedBlockCount*b.N), "blocks/op")
}
