package icons

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"
)

// An ICO contains independently encoded images for different Windows sizes.
// Check that its 256px artwork is the same as the PNG used as its source.
func TestIconSourcesMatch(t *testing.T) {
	tests := []struct {
		name     string
		ico, png []byte
	}{{"base", BaseICO, BasePNG}}
	collectorICO, err := os.ReadFile("collector.ico")
	if err != nil {
		t.Fatal(err)
	}
	collectorPNG, err := os.ReadFile("collector.png")
	if err != nil {
		t.Fatal(err)
	}
	tests = append(tests, struct {
		name     string
		ico, png []byte
	}{"collector", collectorICO, collectorPNG})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.ico
			if len(data) < 6 || binary.LittleEndian.Uint16(data[:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
				t.Fatal("invalid ICO header")
			}
			count := int(binary.LittleEndian.Uint16(data[4:6]))
			if count < 7 || len(data) < 6+16*count {
				t.Fatal("missing icon sizes")
			}
			var png []byte
			sizes := map[int]bool{}
			for i := 0; i < count; i++ {
				entry := data[6+i*16 : 6+(i+1)*16]
				width, height := int(entry[0]), int(entry[1])
				if width == 0 {
					width = 256
				}
				if height == 0 {
					height = 256
				}
				if width != height {
					t.Fatalf("non-square icon: %dx%d", width, height)
				}
				sizes[width] = true
				size, offset := uint64(binary.LittleEndian.Uint32(entry[8:12])), uint64(binary.LittleEndian.Uint32(entry[12:16]))
				if offset+size > uint64(len(data)) {
					t.Fatal("invalid ICO data offset")
				}
				if width == 256 {
					png = data[int(offset):int(offset+size)]
				}
			}
			for _, size := range []int{16, 24, 32, 48, 64, 128, 256} {
				if !sizes[size] {
					t.Fatalf("size %d missing", size)
				}
			}
			got, _, err := image.Decode(bytes.NewReader(png))
			if err != nil {
				t.Fatal(err)
			}
			want, _, err := image.Decode(bytes.NewReader(tc.png))
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds() != want.Bounds() {
				t.Fatalf("bounds differ: %v vs %v", got.Bounds(), want.Bounds())
			}
			for y := 0; y < got.Bounds().Dy(); y++ {
				for x := 0; x < got.Bounds().Dx(); x++ {
					r1, g1, b1, a1 := got.At(x, y).RGBA()
					r2, g2, b2, a2 := want.At(x, y).RGBA()
					if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
						t.Fatalf("pixel differs at %d,%d", x, y)
					}
				}
			}
		})
	}
}

func TestWindowsResourceObjectsMatchIcons(t *testing.T) {
	collectorICO, err := os.ReadFile("collector.ico")
	if err != nil {
		t.Fatal(err)
	}
	arches := []struct {
		name    string
		machine uint16
	}{{"386", pe.IMAGE_FILE_MACHINE_I386}, {"amd64", pe.IMAGE_FILE_MACHINE_AMD64}, {"arm64", pe.IMAGE_FILE_MACHINE_ARM64}}
	for _, program := range []struct {
		name string
		ico  []byte
	}{{"gorig-base", BaseICO}, {"gorig-collector", collectorICO}} {
		for _, arch := range arches {
			t.Run(program.name+"/"+arch.name, func(t *testing.T) {
				path := filepath.Join("..", "..", "cmd", program.name, "rsrc_windows_"+arch.name+".syso")
				file, err := pe.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if file.Machine != arch.machine {
					t.Fatalf("unexpected architecture: %x", file.Machine)
				}
				section := file.Section(".rsrc")
				if section == nil {
					t.Fatal("missing .rsrc section")
				}
				contents, err := section.Data()
				if err != nil {
					t.Fatal(err)
				}
				count := int(binary.LittleEndian.Uint16(program.ico[4:6]))
				for i := 0; i < count; i++ {
					entry := program.ico[6+i*16 : 6+(i+1)*16]
					size, offset := int(binary.LittleEndian.Uint32(entry[8:12])), int(binary.LittleEndian.Uint32(entry[12:16]))
					if size < 0 || offset < 0 || offset > len(program.ico) || size > len(program.ico)-offset {
						t.Fatal("invalid icon offsets")
					}
					if !bytes.Contains(contents, program.ico[offset:offset+size]) {
						t.Fatalf("icon image %d missing from %s; regenerate resources", i, path)
					}
				}
			})
		}
	}
}
