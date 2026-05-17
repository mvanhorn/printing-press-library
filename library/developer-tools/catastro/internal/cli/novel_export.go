// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel features: export parcels, wms layers, srs list.

package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	WFSParcelBase   = "https://ovc.catastro.meh.es/INSPIRE/wfsCP.aspx"
	WFSAddressBase  = "https://ovc.catastro.meh.es/INSPIRE/wfsAD.aspx"
	WFSBuildingBase = "https://ovc.catastro.meh.es/INSPIRE/wfsBU.aspx"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Exporta features INSPIRE (parcelas, direcciones, edificios) por bbox/RC/polígono",
		Long:  "Comandos para exportar features de los servicios INSPIRE WFS del Catastro como GeoJSON/GML.",
	}
	cmd.AddCommand(newExportParcelsCmd(flags))
	cmd.AddCommand(newExportAddressesCmd(flags))
	cmd.AddCommand(newExportBuildingsCmd(flags))
	return cmd
}

func newExportParcelsCmd(flags *rootFlags) *cobra.Command {
	var (
		bbox      string
		rc        string
		polygon   string
		zoning    string
		neighbors bool
		toFile    string
		format    string
		epsg      string
	)
	cmd := &cobra.Command{
		Use:   "parcels",
		Short: "Exporta parcelas (CP.CadastralParcel) por bbox, RC, polígono o zoning",
		Long: "Llama al servicio INSPIRE WFS de Catastro y descarga las parcelas en el filtro seleccionado.\n\n" +
			"Filtros mutuamente exclusivos:\n" +
			"  --bbox X1,Y1,X2,Y2   (rectángulo, recomendado para áreas pequeñas)\n" +
			"  --rc <RC>            (una parcela)\n" +
			"  --polygon ./file.geojson  (recorte por polígono — opera tras descarga por bbox envolvente)\n" +
			"  --zoning <code>      (zoning catastral)",
		Example: "  catastro-pp-cli export parcels --bbox -3.71,40.41,-3.70,40.42 --to parcels.gml",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			filterCount := 0
			for _, f := range []string{bbox, rc, polygon, zoning} {
				if f != "" {
					filterCount++
				}
			}
			if filterCount == 0 {
				return cmd.Help()
			}
			if filterCount > 1 {
				return fmt.Errorf("usa un solo filtro: --bbox | --rc | --polygon | --zoning")
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(),
					`{"would_call":"WFS GetFeature","layer":"CP.CadastralParcel","bbox":%q,"rc":%q,"polygon":%q,"zoning":%q}`+"\n",
					bbox, rc, polygon, zoning)
				return nil
			}

			q := url.Values{}
			q.Set("service", "wfs")
			q.Set("version", "2")
			q.Set("request", "getfeature")
			q.Set("STOREDQUERIE_ID", "GetParcel")
			q.Set("srsname", epsg)

			switch {
			case bbox != "":
				q.Set("bbox", bbox+","+epsg)
				q.Set("TYPENAMES", "cp:CadastralParcel")
				q.Del("STOREDQUERIE_ID")
			case rc != "":
				q.Set("refcat", rc)
				q.Set("TYPENAMES", "cp:CadastralParcel")
			case zoning != "":
				q.Set("TYPENAMES", "cp:CadastralZoning")
				q.Set("filter", fmt.Sprintf(`<Filter><PropertyIsEqualTo><PropertyName>cp:nationalCadastralZoningReference</PropertyName><Literal>%s</Literal></PropertyIsEqualTo></Filter>`, zoning))
			case polygon != "":
				// Use the polygon's bbox as a first pass; full point-in-polygon needs
				// post-processing on the downloaded GML.
				poly, err := os.ReadFile(polygon)
				if err != nil {
					return fmt.Errorf("read polygon: %w", err)
				}
				bb, err := geojsonBBox(poly)
				if err != nil {
					return err
				}
				q.Set("bbox", bb+","+epsg)
				q.Set("TYPENAMES", "cp:CadastralParcel")
				q.Del("STOREDQUERIE_ID")
			}
			_ = neighbors // future flag

			full := WFSParcelBase + "?" + q.Encode()
			body, err := httpGetSimple(full)
			if err != nil {
				return fmt.Errorf("WFS request: %w", err)
			}
			if toFile != "" {
				if err := os.WriteFile(toFile, body, 0644); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), `{"wrote":%q,"bytes":%d,"format":%q}`+"\n", toFile, len(body), format)
				return nil
			}
			if format == "json" || flags.asJSON {
				// Convert GML headers to a minimal JSON envelope so agents can pipe it.
				out := map[string]any{
					"format":  "gml-raw",
					"bytes":   len(body),
					"snippet": truncate(string(body), 500),
					"hint":    "GML output is heavy; redirect with --to <file.gml> for downstream tooling",
				}
				return printJSONFiltered(cmd.OutOrStdout(), mustMarshal(out), flags)
			}
			_, _ = cmd.OutOrStdout().Write(body)
			return nil
		},
	}
	cmd.Flags().StringVar(&bbox, "bbox", "", "Bbox X1,Y1,X2,Y2 (en --epsg)")
	cmd.Flags().StringVar(&rc, "rc", "", "Una sola referencia catastral")
	cmd.Flags().StringVar(&polygon, "polygon", "", "GeoJSON con el polígono (clip por bbox envolvente)")
	cmd.Flags().StringVar(&zoning, "zoning", "", "Código de zoning catastral")
	cmd.Flags().BoolVar(&neighbors, "include-neighbors", false, "Incluir vecinas (reservado)")
	cmd.Flags().StringVar(&toFile, "to", "", "Ruta de salida (default: stdout)")
	cmd.Flags().StringVar(&format, "format", "gml", "Formato de salida: gml | json (resumen)")
	cmd.Flags().StringVar(&epsg, "epsg", "EPSG:4326", "Sistema de referencia espacial (EPSG:25830 común para CAD en España)")
	return cmd
}

func newExportAddressesCmd(flags *rootFlags) *cobra.Command {
	var (
		bbox       string
		rc         string
		codvia     string
		postalcode string
		toFile     string
	)
	cmd := &cobra.Command{
		Use:   "addresses",
		Short: "Exporta direcciones INSPIRE (AD.Address) por bbox/RC/código de vía/CP",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			filterCount := 0
			for _, f := range []string{bbox, rc, codvia, postalcode} {
				if f != "" {
					filterCount++
				}
			}
			if filterCount == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"would_call":"WFS GetFeature","layer":"AD.Address"}`)
				return nil
			}
			q := url.Values{}
			q.Set("service", "wfs")
			q.Set("version", "2")
			q.Set("request", "getfeature")
			q.Set("TYPENAMES", "ad:Address")
			q.Set("srsname", "EPSG:4326")
			if bbox != "" {
				q.Set("bbox", bbox+",EPSG:4326")
			}
			if rc != "" {
				q.Set("refcat", rc)
			}
			if codvia != "" {
				q.Set("codigovia", codvia)
			}
			if postalcode != "" {
				q.Set("postalcode", postalcode)
			}
			body, err := httpGetSimple(WFSAddressBase + "?" + q.Encode())
			if err != nil {
				return err
			}
			if toFile != "" {
				return os.WriteFile(toFile, body, 0644)
			}
			_, _ = cmd.OutOrStdout().Write(body)
			return nil
		},
	}
	cmd.Flags().StringVar(&bbox, "bbox", "", "Bbox X1,Y1,X2,Y2")
	cmd.Flags().StringVar(&rc, "rc", "", "Una referencia catastral")
	cmd.Flags().StringVar(&codvia, "codvia", "", "Código de vía")
	cmd.Flags().StringVar(&postalcode, "postalcode", "", "Código postal")
	cmd.Flags().StringVar(&toFile, "to", "", "Ruta de salida")
	return cmd
}

func newExportBuildingsCmd(flags *rootFlags) *cobra.Command {
	var (
		bbox   string
		rc     string
		toFile string
	)
	cmd := &cobra.Command{
		Use:   "buildings",
		Short: "Exporta edificios INSPIRE (BU.Building) por bbox o RC",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if bbox == "" && rc == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"would_call":"WFS GetFeature","layer":"BU.Building"}`)
				return nil
			}
			q := url.Values{}
			q.Set("service", "wfs")
			q.Set("version", "2")
			q.Set("request", "getfeature")
			q.Set("TYPENAMES", "bu:Building")
			q.Set("srsname", "EPSG:4326")
			if bbox != "" {
				q.Set("bbox", bbox+",EPSG:4326")
			}
			if rc != "" {
				q.Set("refcat", rc)
			}
			body, err := httpGetSimple(WFSBuildingBase + "?" + q.Encode())
			if err != nil {
				return err
			}
			if toFile != "" {
				return os.WriteFile(toFile, body, 0644)
			}
			_, _ = cmd.OutOrStdout().Write(body)
			return nil
		},
	}
	cmd.Flags().StringVar(&bbox, "bbox", "", "Bbox X1,Y1,X2,Y2")
	cmd.Flags().StringVar(&rc, "rc", "", "Una referencia catastral")
	cmd.Flags().StringVar(&toFile, "to", "", "Ruta de salida")
	return cmd
}

// httpGetSimple is a thin HTTP GET with timeout that returns the raw body.
func httpGetSimple(u string) ([]byte, error) {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "catastro-pp-cli")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

// geojsonBBox returns "minX,minY,maxX,maxY" for a GeoJSON polygon/multipolygon.
func geojsonBBox(data []byte) (string, error) {
	var feat map[string]any
	if err := json.Unmarshal(data, &feat); err != nil {
		return "", err
	}
	var coords any
	if g, ok := feat["geometry"].(map[string]any); ok {
		coords = g["coordinates"]
	} else if g, ok := feat["coordinates"]; ok {
		coords = g
	}
	if coords == nil {
		return "", fmt.Errorf("no coordinates found in GeoJSON")
	}
	minX, minY := 1e9, 1e9
	maxX, maxY := -1e9, -1e9
	walkCoords(coords, func(x, y float64) {
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	})
	if minX > maxX {
		return "", fmt.Errorf("invalid geojson: no points found")
	}
	return fmt.Sprintf("%f,%f,%f,%f", minX, minY, maxX, maxY), nil
}

func walkCoords(v any, fn func(x, y float64)) {
	switch t := v.(type) {
	case []any:
		if len(t) >= 2 {
			x, okx := t[0].(float64)
			y, oky := t[1].(float64)
			if okx && oky {
				fn(x, y)
				return
			}
		}
		for _, sub := range t {
			walkCoords(sub, fn)
		}
	}
}

// --- wms layers / srs list ---

func newWMSCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wms",
		Short: "Servicios WMS INSPIRE de Catastro: listado de capas y rendering",
	}
	cmd.AddCommand(newWMSLayersCmd(flags))
	return cmd
}

func newWMSLayersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "layers",
		Short: "Lista las capas WMS disponibles (CP.CadastralParcel, AD.Address, BU.Building, etc.)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"would_call":"WMS GetCapabilities"}`)
				return nil
			}
			body, err := httpGetSimple("https://ovc.catastro.meh.es/cartografia/INSPIRE/spadgcwms.aspx?request=GetCapabilities&service=WMS")
			if err != nil {
				return err
			}
			// Quick-and-dirty extraction of <Name> elements
			layers := []string{}
			type entry struct {
				XMLName xml.Name `xml:"Name"`
				Value   string   `xml:",chardata"`
			}
			dec := xml.NewDecoder(strings.NewReader(string(body)))
			for {
				tok, err := dec.Token()
				if err != nil {
					break
				}
				if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "Name" {
					var v string
					if err := dec.DecodeElement(&v, &se); err == nil {
						v = strings.TrimSpace(v)
						if v != "" && v != "WMS" {
							layers = append(layers, v)
						}
					}
				}
			}
			out := map[string]any{
				"layers": layers,
				"source": "https://ovc.catastro.meh.es/cartografia/INSPIRE/spadgcwms.aspx",
			}
			return printJSONFiltered(cmd.OutOrStdout(), mustMarshal(out), flags)
		},
	}
	return cmd
}

func newSRSCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "srs",
		Short: "Sistemas de referencia espacial soportados por Catastro",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Lista los SRS soportados (EPSG codes) por OVCCoordenadas / WMS / WFS",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			items := []map[string]string{
				{"code": "EPSG:4326", "name": "WGS84 (lon, lat)", "scope": "geocoding global"},
				{"code": "EPSG:25828", "name": "ETRS89 / UTM zone 28N", "scope": "Canarias (oeste)"},
				{"code": "EPSG:25829", "name": "ETRS89 / UTM zone 29N", "scope": "España peninsular oeste (Galicia, Extremadura)"},
				{"code": "EPSG:25830", "name": "ETRS89 / UTM zone 30N", "scope": "España peninsular centro (Madrid, Castilla)"},
				{"code": "EPSG:25831", "name": "ETRS89 / UTM zone 31N", "scope": "España peninsular este (Cataluña, Baleares)"},
				{"code": "EPSG:32628", "name": "WGS84 / UTM zone 28N", "scope": "Canarias"},
				{"code": "EPSG:3857", "name": "Web Mercator", "scope": "Map tiles (Leaflet, Mapbox)"},
			}
			return printJSONFiltered(cmd.OutOrStdout(), mustMarshal(items), flags)
		},
	})
	return cmd
}

// --- cache management ---

func newCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspecciona o limpia el cache local de Catastro",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Imprime la ruta del store/cache local",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := map[string]any{
				"db_path":   defaultDBPath("catastro-pp-cli"),
				"cache_dir": cacheDir(),
			}
			return printJSONFiltered(cmd.OutOrStdout(), mustMarshal(out), flags)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Borra el cache HTTP local (no toca el store de sync)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"would_remove":%q}`+"\n", cacheDir())
				return nil
			}
			if err := os.RemoveAll(cacheDir()); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cleared %s\n", cacheDir())
			return nil
		},
	})
	return cmd
}

func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./.catastro-cache"
	}
	return home + "/.cache/catastro-pp-cli"
}
