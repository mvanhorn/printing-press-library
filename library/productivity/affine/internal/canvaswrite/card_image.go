package canvaswrite

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/socketio"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/yjs"
)

type CardImageOptions struct {
	WorkspaceID string
	DocID       string
	CardID      string
	SourceID    string
	Alt         string
	Width       int
	Height      int
}

type CardImageResult struct {
	DocID           string `json:"doc_id"`
	CardID          string `json:"card_id"`
	ImageID         string `json:"image_id"`
	SourceID        string `json:"source_id"`
	RemovedFallback int    `json:"removed_fallback"`
	UpdatedExisting bool   `json:"updated_existing"`
}

func SetCardImage(cfg *config.Config, opts CardImageOptions) (CardImageResult, error) {
	if opts.WorkspaceID == "" {
		return CardImageResult{}, fmt.Errorf("--workspace is required")
	}
	if opts.DocID == "" || opts.CardID == "" || opts.SourceID == "" {
		return CardImageResult{}, fmt.Errorf("--doc, --card and --source-id are required")
	}
	if opts.Width == 0 {
		opts.Width = 120
	}
	if opts.Height == 0 {
		opts.Height = 90
	}

	bearer := cfg.AffineToken
	if bearer == "" {
		bearer = strings.TrimPrefix(cfg.AuthHeader(), "Bearer ")
	}
	client, err := socketio.Connect(socketio.WSURLFromGraphQL(strings.TrimRight(cfg.BaseURL, "/")+"/graphql"), "", bearer)
	if err != nil {
		return CardImageResult{}, err
	}
	defer client.Close()
	if err := joinWorkspace(client, opts.WorkspaceID); err != nil {
		return CardImageResult{}, err
	}

	engine, err := yjs.NewEngine()
	if err != nil {
		return CardImageResult{}, err
	}
	loaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return CardImageResult{}, err
	}
	if loaded.Missing == "" {
		return CardImageResult{}, fmt.Errorf("document returned empty snapshot")
	}
	doc, err := engine.ApplyBase64Update(loaded.Missing)
	if err != nil {
		return CardImageResult{}, err
	}
	if err := ensureEngineDocIntegrity(engine, doc, opts.DocID, "before card image update"); err != nil {
		return CardImageResult{}, err
	}
	stateVector, err := engine.SaveStateVector(doc)
	if err != nil {
		return CardImageResult{}, err
	}

	imageID := "img-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	script := fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[%d];
			var blocks = doc.getMap("blocks");
			var card = blocks.get(%s);
			if (!(card instanceof Y.Map)) throw new Error("card not found");
			if (card.get("sys:flavour") !== "affine:note") throw new Error("only affine:note cards are supported");

			var children = card.get("sys:children");
			if (!(children instanceof Y.Array)) {
				children = new Y.Array();
				card.set("sys:children", children);
			}

			var removed = 0;
			var updated = false;
			var alt = %s;
			for (var i = children.length - 1; i >= 0; i--) {
				var childId = children.get(i);
				var child = blocks.get(childId);
				if (!(child instanceof Y.Map)) continue;
				var flavour = child.get("sys:flavour");
				if (flavour === "affine:image") {
					if (!updated) {
						child.set("prop:sourceId", %s);
						child.set("prop:width", %d);
						child.set("prop:height", %d);
						child.set("prop:caption", "");
						child.set("prop:size", -1);
						updated = true;
					} else {
						children.delete(i, 1);
					}
					continue;
				}
				var text = child.get("prop:text");
				if (text instanceof Y.Text) {
					var s = text.toString().trim();
					if (s === "!" + alt || s === "![" + alt + "]") {
						children.delete(i, 1);
						blocks.delete(childId);
						removed++;
					}
				}
			}

			var imageId = %s;
			if (!updated) {
				var image = new Y.Map();
				image.set("sys:id", imageId);
				image.set("sys:flavour", "affine:image");
				image.set("sys:version", 1);
				image.set("sys:children", new Y.Array());
				image.set("prop:sourceId", %s);
				image.set("prop:width", %d);
				image.set("prop:height", %d);
				image.set("prop:rotate", 0);
				image.set("prop:size", -1);
				image.set("prop:caption", "");
				image.set("prop:textAlign", undefined);
				blocks.set(imageId, image);
				children.insert(0, [imageId]);
			}
			return JSON.stringify({image_id: updated ? "" : imageId, removed_fallback: removed, updated_existing: updated});
		})()
	`, doc, strconv.Quote(opts.CardID), strconv.Quote(opts.Alt), strconv.Quote(opts.SourceID), opts.Width, opts.Height, strconv.Quote(imageID), strconv.Quote(opts.SourceID), opts.Width, opts.Height)
	raw, err := engine.RunScript(script)
	if err != nil {
		return CardImageResult{}, err
	}
	if err := ensureEngineDocIntegrity(engine, doc, opts.DocID, "after local card image update"); err != nil {
		return CardImageResult{}, err
	}
	delta, err := engine.EncodeDelta(doc, stateVector)
	if err != nil {
		return CardImageResult{}, err
	}
	if err := client.PushDocUpdate(opts.WorkspaceID, opts.DocID, delta); err != nil {
		return CardImageResult{}, err
	}
	reloaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return CardImageResult{}, err
	}
	reloadedDoc, err := engine.ApplyBase64Update(reloaded.Missing)
	if err != nil {
		return CardImageResult{}, err
	}
	if err := ensureEngineDocIntegrity(engine, reloadedDoc, opts.DocID, "after pushed card image update"); err != nil {
		return CardImageResult{}, err
	}

	result := CardImageResult{
		DocID:    opts.DocID,
		CardID:   opts.CardID,
		ImageID:  imageID,
		SourceID: opts.SourceID,
	}
	var patch struct {
		ImageID         string `json:"image_id"`
		RemovedFallback int    `json:"removed_fallback"`
		UpdatedExisting bool   `json:"updated_existing"`
	}
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		return CardImageResult{}, fmt.Errorf("parse patch result: %w", err)
	}
	if patch.ImageID != "" {
		result.ImageID = patch.ImageID
	}
	result.RemovedFallback = patch.RemovedFallback
	result.UpdatedExisting = patch.UpdatedExisting
	return result, nil
}
