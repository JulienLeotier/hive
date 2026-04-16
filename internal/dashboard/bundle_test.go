package dashboard

import (
	"io/fs"
	"strings"
	"testing"
)

// TestBundleSizeBudget guards Story 8.1 SLA ("dashboard loads in under 2 seconds").
// Load time depends on network + device, but a bundle-size budget is the
// enforcement point we can measure in CI. Budget:
//   - total JS+CSS ≤ 1 MB (comfortable over 3G even with 0.5s TTFB)
//   - total HTML + assets ≤ 3 MB (images, fonts)
func TestBundleSizeBudget(t *testing.T) {
	var jsCss int64
	var total int64

	err := fs.WalkDir(assets, "dist", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
			jsCss += info.Size()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking dashboard bundle: %v", err)
	}

	t.Logf("dashboard bundle: total=%d B, js+css=%d B", total, jsCss)

	const jsCssBudget = 1 * 1024 * 1024      // 1 MB
	const totalBudget = 3 * 1024 * 1024      // 3 MB

	if jsCss > jsCssBudget {
		t.Fatalf("js+css bundle %d B exceeds %d B budget (Story 8.1 2s load SLA)", jsCss, jsCssBudget)
	}
	if total > totalBudget {
		t.Fatalf("total bundle %d B exceeds %d B budget (Story 8.1 2s load SLA)", total, totalBudget)
	}
}
