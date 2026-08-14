package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

const sensitiveFieldMax = 200000

var fieldsToExpand = []string{"dob", "ssn", "participantSigDataUrl", "casemanagerSigDataUrl"}

// upExpandSensitiveFieldMax increases the max length of sensitive text fields
// so AES-GCM ciphertext (with its version prefix and base64) fits.
func upExpandSensitiveFieldMax(app core.App) error {
	col, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return fmt.Errorf("find intake collection: %w", err)
	}
	changed := false
	for _, name := range fieldsToExpand {
		f := col.Fields.GetByName(name)
		if f == nil {
			continue
		}
		tf, ok := f.(*core.TextField)
		if !ok {
			continue
		}
		if tf.Max < sensitiveFieldMax {
			tf.Max = sensitiveFieldMax
			changed = true
		}
	}
	if changed {
		if err := app.Save(col); err != nil {
			return fmt.Errorf("save intake collection: %w", err)
		}
	}
	return nil
}

// downExpandSensitiveFieldMax reverts the max length change. Only safe when no
// encrypted values exceed the original limits.
func downExpandSensitiveFieldMax(app core.App) error {
	col, err := app.FindCollectionByNameOrId("intake")
	if err != nil {
		return fmt.Errorf("find intake collection: %w", err)
	}
	changed := false
	for _, name := range fieldsToExpand {
		f := col.Fields.GetByName(name)
		if f == nil {
			continue
		}
		tf, ok := f.(*core.TextField)
		if !ok {
			continue
		}
		if tf.Max == sensitiveFieldMax {
			tf.Max = 20
			changed = true
		}
		if name == "participantSigDataUrl" || name == "casemanagerSigDataUrl" {
			tf.Max = 100000
		}
	}
	if changed {
		if err := app.Save(col); err != nil {
			return fmt.Errorf("save intake collection: %w", err)
		}
	}
	return nil
}
