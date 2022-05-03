package transform

//func TestNewEthAckIndexProvider(t *testing.T) {
//	indexStore := dstore.NewMockStore(func(base string, f io.Reader) error {
//		return nil
//	})
//	indexProvider := NewEthLogIndexProvider(indexStore, nil, nil)
//	require.NotNil(t, indexProvider)
//	require.IsType(t, transform.GenericBlockIndexProvider{}, *indexProvider)
//}
//
//func TestEthAckIndexProvider_WithinRange(t *testing.T) {
//	tests := []struct {
//		name          string
//		blocks        []*pbeth.Block
//		indexSize     uint64
//		wantedBlock   uint64
//		isWithinRange bool
//	}{
//		{
//			name:          "block exists in first index",
//			blocks:        testEthBlocks(t, 5),
//			indexSize:     2,
//			wantedBlock:   11,
//			isWithinRange: true,
//		},
//		{
//			name:          "block exists in second index",
//			blocks:        testEthBlocks(t, 5),
//			indexSize:     2,
//			wantedBlock:   13,
//			isWithinRange: true,
//		},
//		{
//			name:          "block doesn't exist",
//			blocks:        testEthBlocks(t, 5),
//			indexSize:     2,
//			wantedBlock:   69,
//			isWithinRange: false,
//		},
//	}
//
//	matchAddresses := []eth.Address{eth.Address("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			// populate a mock dstore with some index files
//			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)
//
//			// spawn an indexProvider with the populated dstore
//			indexProvider := NewEthLogIndexProvider(
//				indexStore,
//				[]uint64{test.indexSize},
//				[]*addrSigSingleFilter{
//					{matchAddresses, nil},
//				},
//			)
//			require.NotNil(t, indexProvider)
//
//			// meat and potatoes
//			b := indexProvider.WithinRange(context.Background(), test.wantedBlock)
//			if test.isWithinRange {
//				require.True(t, b)
//			} else {
//				require.False(t, b)
//			}
//		})
//	}
//}
//
//func TestEthLogIndexProvider_Matches(t *testing.T) {
//	tests := []struct {
//		name            string
//		blocks          []*pbeth.Block
//		indexSize       uint64
//		wantedBlock     uint64
//		expectMatches   bool
//		filterAddresses []eth.Address
//		filterEventSigs []eth.Hash
//	}{
//		{
//			name:          "matches single address",
//			blocks:        testEthBlocks(t, 5),
//			indexSize:     2,
//			wantedBlock:   11,
//			expectMatches: true,
//			filterAddresses: []eth.Address{
//				eth.MustNewAddress("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
//			},
//			filterEventSigs: []eth.Hash{},
//		},
//		{
//			name:          "doesn't match single address",
//			blocks:        testEthBlocks(t, 5),
//			indexSize:     2,
//			wantedBlock:   11,
//			expectMatches: false,
//			filterAddresses: []eth.Address{
//				eth.MustNewAddress("efefefefefefefefefefefefefefefefefefefef"),
//			},
//			filterEventSigs: []eth.Hash{},
//		},
//		{
//			name:            "matches single event sig",
//			blocks:          testEthBlocks(t, 5),
//			indexSize:       2,
//			wantedBlock:     10,
//			expectMatches:   true,
//			filterAddresses: []eth.Address{},
//			filterEventSigs: []eth.Hash{
//				eth.MustNewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
//			},
//		},
//		{
//			name:            "doesn't match single event sig",
//			blocks:          testEthBlocks(t, 5),
//			indexSize:       2,
//			wantedBlock:     10,
//			expectMatches:   false,
//			filterAddresses: []eth.Address{},
//			filterEventSigs: []eth.Hash{
//				eth.MustNewHash("efefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefef"),
//			},
//		},
//		{
//			name:          "matches multi address multi sig",
//			blocks:        testEthBlocks(t, 5),
//			indexSize:     2,
//			wantedBlock:   13,
//			expectMatches: true,
//			filterAddresses: []eth.Address{
//				eth.MustNewAddress("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
//				eth.MustNewAddress("4444444444444444444444444444444444444444"),
//			},
//			filterEventSigs: []eth.Hash{
//				eth.MustNewHash("3333333333333333333333333333333333333333333333333333333333333333"),
//				eth.MustNewHash("4444444444444444444444444444444444444444444444444444444444444444"),
//			},
//		},
//		{
//			name:          "doesn't match multi address multi sig",
//			blocks:        testEthBlocks(t, 5),
//			indexSize:     2,
//			wantedBlock:   13,
//			expectMatches: false,
//			filterAddresses: []eth.Address{
//				eth.MustNewAddress("beefbeefbeefbeefbeefbeefbeefbeefbeefbeef"),
//				eth.MustNewAddress("deaddeaddeaddeaddeaddeaddeaddeaddeaddead"),
//			},
//			filterEventSigs: []eth.Hash{
//				eth.MustNewHash("efefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefef"),
//				eth.MustNewHash("abababababababababababababababababababababababababababababababab"),
//			},
//		},
//	}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)
//			filters := []*addrSigSingleFilter{
//				{test.filterAddresses, test.filterEventSigs},
//			}
//			indexProvider := NewEthLogIndexProvider(indexStore, []uint64{test.indexSize}, filters)
//
//			b, err := indexProvider.Matches(context.Background(), test.wantedBlock)
//			require.NoError(t, err)
//			if test.expectMatches {
//				require.True(t, b)
//			} else {
//				require.False(t, b)
//			}
//		})
//	}
//}
//
//func TestEthLogIndexProvider_NextMatching(t *testing.T) {
//	tests := []struct {
//		name                        string
//		blocks                      []*pbeth.Block
//		indexSize                   uint64
//		wantedBlock                 uint64
//		expectedNextBlockNum        uint64
//		expectedPassedIndexBoundary bool
//		filterAddresses             []eth.Address
//		filterEventSigs             []eth.Hash
//	}{
//		{
//			name:                        "block exists in first index and filters match block in second index",
//			blocks:                      testEthBlocks(t, 5),
//			indexSize:                   2,
//			wantedBlock:                 11,
//			expectedNextBlockNum:        13,
//			expectedPassedIndexBoundary: false,
//			filterAddresses: []eth.Address{
//				eth.MustNewAddress("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
//			},
//			filterEventSigs: []eth.Hash{
//				eth.MustNewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
//			},
//		},
//		{
//			name:        "block exists in first index and filters match block outside bounds",
//			indexSize:   2,
//			blocks:      testEthBlocks(t, 5),
//			wantedBlock: 10,
//			filterAddresses: []eth.Address{
//				eth.MustNewAddress("cccccccccccccccccccccccccccccccccccccccc"),
//			},
//			filterEventSigs: []eth.Hash{
//				eth.MustNewHash("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
//			},
//			expectedNextBlockNum:        14,
//			expectedPassedIndexBoundary: true,
//		},
//		{
//			name:        "filters don't match any known blocks",
//			indexSize:   2,
//			blocks:      testEthBlocks(t, 5),
//			wantedBlock: 10,
//			filterAddresses: []eth.Address{
//				eth.MustNewAddress("beefbeefbeefbeefbeefbeefbeefbeefbeefbeef"),
//			},
//			filterEventSigs: []eth.Hash{
//				eth.MustNewHash("efefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefef"),
//			},
//			expectedNextBlockNum:        14,
//			expectedPassedIndexBoundary: true,
//		},
//	}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			indexStore := testMockstoreWithFiles(t, test.blocks, test.indexSize)
//			filters := []*addrSigSingleFilter{
//				{test.filterAddresses, test.filterEventSigs},
//			}
//			indexProvider := NewEthLogIndexProvider(indexStore, []uint64{test.indexSize}, filters)
//
//			nextBlockNum, passedIndexBoundary, err := indexProvider.NextMatching(context.Background(), test.wantedBlock, 0)
//			require.NoError(t, err)
//			require.Equal(t, passedIndexBoundary, test.expectedPassedIndexBoundary)
//			require.Equal(t, test.expectedNextBlockNum, nextBlockNum)
//		})
//	}
//}
