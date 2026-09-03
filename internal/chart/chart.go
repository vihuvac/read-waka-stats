package chart

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"

	"github.com/vihuvac/read-waka-stats/internal/commits"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

const maxLanguages = 5

// Path is the default asset path written into the profile repository.
const Path = "assets/bar_graph.png"

func parseHex(s string) color.Color {
	s = trimHash(s)
	var r, g, b uint8
	if len(s) == 6 {
		_, _ = fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
		return color.RGBA{R: r, G: g, B: b, A: 255}
	}
	return color.RGBA{R: 128, G: 128, B: 128, A: 255}
}

func trimHash(s string) string {
	if len(s) > 0 && s[0] == '#' {
		return s[1:]
	}
	return s
}

type yqLang struct {
	add, del int
}

// Draw writes a stacked bar chart of additions (positive) and deletions (negative).
func Draw(yearly commits.YearlyData, colors githubx.LinguistColors, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	p := plot.New()
	p.Title.Text = "Lines of code"
	p.Y.Label.Text = "LOC added"
	p.Legend.Top = true

	years := make([]int, 0, len(yearly))
	for y := range yearly {
		years = append(years, y)
	}
	sort.Ints(years)
	if len(years) == 0 {
		return p.Save(8*vg.Inch, 4*vg.Inch, dest)
	}

	langs := map[string][][4]yqLang{}
	for yi, y := range years {
		for q := 1; q <= 4; q++ {
			bucket := yearly[y][q]
			type pair struct {
				name string
				n    int
			}
			ranked := make([]pair, 0, len(bucket))
			for name, d := range bucket {
				ranked = append(ranked, pair{name, d.Add + d.Del})
			}
			sort.Slice(ranked, func(i, j int) bool { return ranked[i].n > ranked[j].n })
			if len(ranked) > maxLanguages {
				ranked = ranked[:maxLanguages]
			}
			for _, r := range ranked {
				if langs[r.name] == nil {
					langs[r.name] = make([][4]yqLang, len(years))
				}
				d := bucket[r.name]
				langs[r.name][yi][q-1] = yqLang{add: d.Add, del: d.Del}
			}
		}
	}

	langNames := make([]string, 0, len(langs))
	for name := range langs {
		langNames = append(langNames, name)
	}
	sort.Strings(langNames)

	width := len(years) * 4
	var prevAdd, prevDel *plotter.BarChart
	for _, name := range langNames {
		addVals := make(plotter.Values, width)
		delVals := make(plotter.Values, width)
		series := langs[name]
		for yi := 0; yi < len(years); yi++ {
			for q := 0; q < 4; q++ {
				idx := yi*4 + q
				addVals[idx] = float64(series[yi][q].add)
				delVals[idx] = -float64(series[yi][q].del)
			}
		}
		col := parseHex(colors[name])
		ab, err := plotter.NewBarChart(addVals, vg.Points(10))
		if err != nil {
			return err
		}
		ab.Color = col
		ab.LineStyle.Width = 0
		if prevAdd != nil {
			ab.StackOn(prevAdd)
		}
		db, err := plotter.NewBarChart(delVals, vg.Points(10))
		if err != nil {
			return err
		}
		db.Color = col
		db.LineStyle.Width = 0
		if prevDel != nil {
			db.StackOn(prevDel)
		}
		p.Add(ab, db)
		p.Legend.Add(name, ab)
		prevAdd, prevDel = ab, db
	}

	labels := make([]string, width)
	for yi, y := range years {
		for q := 0; q < 4; q++ {
			if q == 0 {
				labels[yi*4+q] = fmt.Sprintf("%d Q1", y)
			} else {
				labels[yi*4+q] = fmt.Sprintf("Q%d", q+1)
			}
		}
	}
	p.NominalX(labels...)
	w := vg.Length(2.5+float64(width)*0.35) * vg.Inch
	return p.Save(w, 4*vg.Inch, dest)
}
