// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package pdfgen

import _ "embed"

// PATCH: embed the extracted Partitaiva24 SPA PDF renderer used by invoices download-pdf.

//go:embed render-pdf.js
var RenderPDFScript []byte

//go:embed assets/bundle.patched.js
var RenderPDFBundle []byte

const BundleGeneratedAt = "2026-05-09"
