package layout

// Region coordinates are cells, so a test that sliced by runes would agree with
// a rune-counting bug and report a pass.
func cellSlice(line string, start, end int) string {
	w := Renderer{}
	var out []rune
	col := 0
	for _, r := range line {
		n := w.Of(string(r))
		if col >= start && col+n <= end {
			out = append(out, r)
		}
		col += n
	}
	return string(out)
}
