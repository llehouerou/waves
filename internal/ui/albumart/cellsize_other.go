//go:build !unix

package albumart

func getCellSize() (cellW, cellH int) {
	return 8, 16
}
