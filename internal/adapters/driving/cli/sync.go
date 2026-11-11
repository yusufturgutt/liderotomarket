// Package cli komut satırından gelen istekleri core servislerine iletir (driving adapter).
package cli

import (
	"context"
	"fmt"
	"io"

	"liderotomarket/internal/core/ports"
)

// RunSync senkronu çalıştırır ve özeti yazdırır.
func RunSync(ctx context.Context, svc ports.SyncService, dryRun bool, out io.Writer) error {
	if dryRun {
		fmt.Fprintln(out, "Deneme modu: hiçbir şey kaydedilmeyecek.")
	}
	rep, err := svc.Sync(ctx, dryRun)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\n✓ %d ürün · %d yeni · %d güncellendi · %d silindi\n", rep.Total, rep.Added, rep.Updated, rep.Removed)
	if rep.DryRun {
		fmt.Fprintf(out, "  %d görsel indirilecek/kontrol edilecek\n", rep.ImagesTotal)
		return nil
	}
	fmt.Fprintf(out, "  %d görsel (%d yeni indirildi, %d eski silindi)", rep.ImagesTotal, rep.ImagesDownloaded, rep.ImagesPruned)
	if rep.ImagesFailed > 0 {
		fmt.Fprintf(out, " · %d görsel indirilemedi", rep.ImagesFailed)
	}
	fmt.Fprintf(out, "\n  Süre: %s\n", rep.Duration.Round(100_000_000))
	return nil
}
