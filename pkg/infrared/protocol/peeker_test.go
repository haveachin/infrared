package protocol

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

func TestBytePeeker_ReadByte(t *testing.T) {
	tt := []struct {
		peeker       bytePeeker
		data         []byte
		expectedByte byte
	}{
		{
			peeker: bytePeeker{
				cursor: 0,
			},
			data:         []byte{0x00, 0x01, 0x02, 0x03},
			expectedByte: 0x00,
		},
		{
			peeker: bytePeeker{
				cursor: 1,
			},
			data:         []byte{0x00, 0x01, 0x02, 0x03},
			expectedByte: 0x01,
		},
		{
			peeker: bytePeeker{
				cursor: 3,
			},
			data:         []byte{0x00, 0x01, 0x02, 0x03},
			expectedByte: 0x03,
		},
	}

	for _, tc := range tt {
		// Arrange
		clonedData := make([]byte, len(tc.data))
		copy(clonedData, tc.data)
		tc.peeker.PeekReader = bufio.NewReader(bytes.NewReader(clonedData))

		// Act
		b, err := tc.peeker.ReadByte()
		if err != nil && err != io.EOF {
			t.Error(err)
		}

		// Assert
		if b != tc.expectedByte {
			t.Errorf("got: %v; want: %v", b, tc.expectedByte)
		}

		if !bytes.Equal(clonedData, tc.data) {
			t.Errorf("data modified: got: %v; want: %v", clonedData, tc.data)
		}
	}
}

func TestBytePeeker_Read(t *testing.T) {
	tt := []struct {
		peeker       bytePeeker
		data         []byte
		expectedData []byte
		expectedN    int
	}{
		{
			peeker: bytePeeker{
				cursor: 0,
			},
			data:         []byte{0x00, 0x01, 0x02, 0x03},
			expectedData: []byte{0x00, 0x01, 0x02, 0x03},
			expectedN:    4,
		},
		{
			peeker: bytePeeker{
				cursor: 1,
			},
			data:         []byte{0x00, 0x01, 0x02, 0x03},
			expectedData: []byte{0x01, 0x02, 0x03},
			expectedN:    3,
		},
		{
			peeker: bytePeeker{
				cursor: 3,
			},
			data:         []byte{0x00, 0x01, 0x02, 0x03},
			expectedData: []byte{0x03},
			expectedN:    1,
		},
	}

	for _, tc := range tt {
		// Arrange
		clonedData := make([]byte, len(tc.data))
		copy(clonedData, tc.data)
		tc.peeker.PeekReader = bufio.NewReader(bytes.NewReader(clonedData))
		resultData := make([]byte, len(tc.expectedData))

		// Act
		n, err := tc.peeker.Read(resultData)
		if err != nil && err != io.EOF {
			t.Error(err)
		}

		// Assert
		if n != tc.expectedN {
			t.Errorf("got: %v; want: %v", n, tc.expectedN)
		}

		if !bytes.Equal(resultData, tc.expectedData) {
			t.Errorf("got: %v; want: %v", resultData, tc.expectedData)
		}

		if !bytes.Equal(clonedData, tc.data) {
			t.Errorf("data modified: got: %v; want: %v", clonedData, tc.data)
		}
	}
}
