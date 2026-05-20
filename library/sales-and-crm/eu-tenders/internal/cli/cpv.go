// Copyright 2026 mathias-michel. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// CPVEntry is a CPV code and its description.
type CPVEntry struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// cpvDivisions contains the most common CPV sections and divisions.
// pp:novel-static-reference
var cpvDivisions = []CPVEntry{
	{"03000000", "Agricultural, farming, fishing, forestry and related products"},
	{"03100000", "Agricultural and horticultural products"},
	{"03200000", "Cereals, potatoes, vegetables, fruits and nuts"},
	{"03400000", "Forestry and logging products"},
	{"09000000", "Petroleum products, fuel, electricity and other sources of energy"},
	{"09100000", "Fuels"},
	{"09200000", "Petroleum, coal and oil products"},
	{"09300000", "Electricity, heating, solar and nuclear energy"},
	{"14000000", "Mining, basic metals and related products"},
	{"15000000", "Food, beverages, tobacco and related products"},
	{"15100000", "Animal products, meat and meat products"},
	{"15200000", "Fish and other water animals"},
	{"15300000", "Fruit, vegetables and related products"},
	{"15800000", "Miscellaneous food products"},
	{"18000000", "Clothing, footwear, luggage articles and accessories"},
	{"22000000", "Printed matter and related products"},
	{"22100000", "Printed books, brochures and leaflets"},
	{"22400000", "Stamps, cheque forms, banknotes, stock certificates"},
	{"24000000", "Chemical products"},
	{"24100000", "Gases"},
	{"24200000", "Dyes and pigments"},
	{"24900000", "Fine and various chemical products"},
	{"30000000", "Office and computing machinery, equipment and supplies"},
	{"30100000", "Office machinery, equipment and supplies"},
	{"30200000", "Computer equipment and supplies"},
	{"31000000", "Electrical machinery, apparatus, equipment and consumables"},
	{"31100000", "Electric motors, generators and transformers"},
	{"31500000", "Lighting equipment and electric lamps"},
	{"32000000", "Radio, television, communication, telecommunication equipment"},
	{"32200000", "Transmitting apparatus for radio-telephony, radio-telegraphy"},
	{"32400000", "Networks"},
	{"32500000", "Telecommunications equipment and supplies"},
	{"33000000", "Medical equipments, pharmaceuticals and personal care products"},
	{"33100000", "Medical equipments"},
	{"33600000", "Pharmaceutical products"},
	{"34000000", "Transport equipment and auxiliary products to transportation"},
	{"34100000", "Motor vehicles"},
	{"34200000", "Vehicle bodies, trailers or semi-trailers"},
	{"34900000", "Miscellaneous transport equipment and spare parts"},
	{"38000000", "Laboratory, optical and precision instruments, equipment"},
	{"39000000", "Furniture, household goods, appliances and supplies"},
	{"39100000", "Furniture"},
	{"39700000", "Domestic appliances"},
	{"41000000", "Collected and purified water"},
	{"42000000", "Industrial machinery"},
	{"43000000", "Machinery for mining, quarrying, construction equipment"},
	{"44000000", "Construction structures and materials, auxiliary construction products"},
	{"45000000", "Construction work"},
	{"45100000", "Site preparation work"},
	{"45200000", "Works for complete or part construction and civil engineering work"},
	{"45210000", "Building construction work"},
	{"45220000", "Engineering works and construction works"},
	{"45230000", "Construction work for pipelines, communication and power lines"},
	{"45300000", "Building installation work"},
	{"45310000", "Electrical installation work"},
	{"45320000", "Insulation work"},
	{"45330000", "Plumbing and sanitary engineering work"},
	{"45340000", "Fencing, railing and safety equipment installation work"},
	{"45400000", "Building completion work"},
	{"45410000", "Plastering work"},
	{"45420000", "Joinery and carpentry installation work"},
	{"45430000", "Floor and wall covering work"},
	{"45440000", "Painting and glazing work"},
	{"45500000", "Hiring of construction and civil engineering machinery and equipment"},
	{"48000000", "Software packages and information systems"},
	{"48100000", "Industry specific software package"},
	{"48200000", "Networking, Internet and intranet software package"},
	{"48400000", "Transaction and personal business software package"},
	{"48500000", "Communication and multimedia software package"},
	{"48600000", "Database and operating software package"},
	{"48700000", "Software package utilities"},
	{"48800000", "Information systems and servers"},
	{"50000000", "Repair and maintenance services"},
	{"50100000", "Repair, maintenance and associated services of vehicles"},
	{"50300000", "Repair, maintenance and associated services of personal computers"},
	{"50700000", "Repair and maintenance services of building installations"},
	{"55000000", "Hotel, restaurant and retail trade services"},
	{"55100000", "Hotel services"},
	{"55300000", "Restaurant and food-serving services"},
	{"60000000", "Transport services (excl. waste transport)"},
	{"60100000", "Road transport services"},
	{"60200000", "Rail transport services"},
	{"60400000", "Air transport services"},
	{"60500000", "Space transport services"},
	{"63000000", "Supporting and auxiliary transport services"},
	{"63100000", "Cargo handling and storage services"},
	{"63500000", "Travel agency, tour operator and tourist assistance services"},
	{"64000000", "Post and telecommunications services"},
	{"64100000", "Post and courier services"},
	{"64200000", "Telecommunications services"},
	{"65000000", "Public utilities"},
	{"65100000", "Water distribution and related services"},
	{"65200000", "Gas distribution and related services"},
	{"65300000", "Electricity distribution and related services"},
	{"66000000", "Financial and insurance services"},
	{"66100000", "Banking and investment services"},
	{"66500000", "Insurance and pension services"},
	{"70000000", "Real estate services"},
	{"70100000", "Real estate services with own property"},
	{"70300000", "Residential property services"},
	{"71000000", "Architectural, construction, engineering and inspection services"},
	{"71200000", "Architectural and related services"},
	{"71300000", "Engineering services"},
	{"71400000", "Urban planning and landscape architectural services"},
	{"71500000", "Construction-related services"},
	{"71600000", "Technical testing, analysis and consultancy services"},
	{"72000000", "IT services: consulting, software development, Internet and support"},
	{"72100000", "Hardware consultancy services"},
	{"72200000", "Software programming and consultancy services"},
	{"72210000", "Programming services of packaged software products"},
	{"72220000", "Systems and technical consultancy services"},
	{"72230000", "Custom software development services"},
	{"72240000", "Systems analysis and programming services"},
	{"72250000", "System and support services"},
	{"72260000", "Software-related services"},
	{"72300000", "Data services"},
	{"72310000", "Data-processing services"},
	{"72320000", "Database services"},
	{"72400000", "Internet services"},
	{"72500000", "Computer-related services"},
	{"72600000", "Computer support and consultancy services"},
	{"72700000", "Computer network services"},
	{"72800000", "Computer audit and testing services"},
	{"72900000", "Computer back-up and catalogue conversion services"},
	{"73000000", "Research and development services"},
	{"73100000", "Research services"},
	{"73200000", "Research and development consultancy services"},
	{"75000000", "Administration, defence and social security services"},
	{"75100000", "Administration services"},
	{"75200000", "Provision of services to the community"},
	{"75300000", "Compulsory social security services"},
	{"76000000", "Services related to the oil and gas industry"},
	{"77000000", "Agricultural, forestry, horticultural, aquaculture and apicultural services"},
	{"77100000", "Agricultural services"},
	{"77200000", "Forestry services"},
	{"77300000", "Horticultural services"},
	{"79000000", "Business services: law, marketing, consulting, recruitment, printing"},
	{"79100000", "Legal services"},
	{"79200000", "Accounting, auditing and fiscal services"},
	{"79300000", "Market and economic research; polling and statistics"},
	{"79400000", "Business and management consultancy and related services"},
	{"79500000", "Office-support services"},
	{"79600000", "Recruitment services"},
	{"79700000", "Investigation and security services"},
	{"79800000", "Printing and related services"},
	{"80000000", "Education and training services"},
	{"80100000", "Primary education services"},
	{"80200000", "Secondary education services"},
	{"80300000", "Higher education services"},
	{"80400000", "Adult and other education services"},
	{"80500000", "Training services"},
	{"85000000", "Health and social work services"},
	{"85100000", "Health services"},
	{"85200000", "Veterinary services"},
	{"85300000", "Social work and related services"},
	{"90000000", "Sewage and refuse disposal services, sanitation and environmental services"},
	{"90100000", "Sewage and refuse disposal services"},
	{"90500000", "Refuse and waste related services"},
	{"90600000", "Cleaning and sanitation services in urban or rural areas"},
	{"90700000", "Environmental services"},
	{"90900000", "Cleaning and sanitation services"},
	{"92000000", "Recreational, cultural and sporting services"},
	{"92100000", "Motion picture and video services"},
	{"92200000", "Radio and television services"},
	{"92300000", "Entertainment services"},
	{"92600000", "Sporting services"},
	{"98000000", "Other community, social and personal services"},
	{"98100000", "Membership organisation services"},
	{"98300000", "Miscellaneous services"},
}

func newCPVCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cpv",
		Short: "Browse and search Common Procurement Vocabulary (CPV) codes",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}

	cmd.AddCommand(newCPVSearchCmd(flags))
	cmd.AddCommand(newCPVGetCmd(flags))

	return cmd
}

func newCPVSearchCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "search <keyword>",
		Short: "Search CPV descriptions for a keyword",
		Long: `Search CPV codes by keyword in their description.

Examples:
  eu-tenders-pp-cli cpv search software
  eu-tenders-pp-cli cpv search construction
  eu-tenders-pp-cli cpv search "health services" --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			kw := strings.ToLower(strings.Join(args, " "))
			var matches []CPVEntry
			for _, e := range cpvDivisions {
				if strings.Contains(strings.ToLower(e.Description), kw) {
					matches = append(matches, e)
				}
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(matches)
			}

			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No CPV codes found for %q\n", kw)
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "CODE\tDESCRIPTION")
			for _, e := range matches {
				fmt.Fprintf(tw, "%s\t%s\n", e.Code, e.Description)
			}
			return tw.Flush()
		},
	}
}

func newCPVGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <code>",
		Short: "Show details for a CPV code",
		Long: `Show the description for a specific CPV code.

Examples:
  eu-tenders-pp-cli cpv get 72000000
  eu-tenders-pp-cli cpv get 45`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			code := normalizeCPV(strings.TrimSpace(args[0]))
			var found *CPVEntry
			for i := range cpvDivisions {
				if cpvDivisions[i].Code == code {
					found = &cpvDivisions[i]
					break
				}
			}

			// Fallback: prefix match.
			if found == nil {
				prefix := strings.TrimRight(code, "0")
				for i := range cpvDivisions {
					if strings.HasPrefix(cpvDivisions[i].Code, prefix) {
						found = &cpvDivisions[i]
						break
					}
				}
			}

			if found == nil {
				return fmt.Errorf("CPV code %s not found in local reference data\nhint: use 'eu-tenders-pp-cli cpv search <keyword>' to browse", code)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(found)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Code:        %s\nDescription: %s\n", found.Code, found.Description)
			return nil
		},
	}
}
