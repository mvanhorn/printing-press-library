// Package refdata is a curated static reference set of NAICS and PSC codes.
// pp:novel-static-reference
//
// The full NAICS table (~1,200 codes) and PSC table (~5,000 codes) live in the
// USAspending /api/v2/references/naics/ endpoint and in published GSA PSC
// manuals respectively, and the printed CLI's `sync --refdata` flow can pull
// them. We ship a curated subset here so `code resolve` works on a fresh
// install without a sync, and so the IT-flavored codes federal-tech users
// reach for daily are always present.

package refdata

// CodeSeed is one row in the curated seed. Used by both NAICS and PSC tables;
// the consumer chooses which slice to load.
type CodeSeed struct {
	Code     string
	Title    string
	Category string // PSC category letter / number group; empty for NAICS
	Parent   string // NAICS parent code; empty for PSC
	Depth    int    // NAICS depth (2,3,4,5,6); 0 for PSC
}

// NAICSSeeds covers the federal-IT-relevant subset of the 2022 NAICS table:
// all of 5415* (Computer Systems Design and Related Services), the data
// processing / cloud subsector (518210, 519130), R&D services (5417*), and
// the engineering services categories most often tagged as IT modernization.
var NAICSSeeds = []CodeSeed{
	// Information sector roots
	{"51", "Information", "", "", 2},
	{"518", "Computing Infrastructure Providers, Data Processing, Web Hosting, and Related Services", "", "51", 3},
	{"5182", "Computing Infrastructure Providers, Data Processing, Web Hosting, and Related Services", "", "518", 4},
	{"51821", "Computing Infrastructure Providers, Data Processing, Web Hosting, and Related Services", "", "5182", 5},
	{"518210", "Computing Infrastructure Providers, Data Processing, Web Hosting, and Related Services", "", "51821", 6},
	{"519", "Web Search Portals, Libraries, Archives, and Other Information Services", "", "51", 3},
	{"5191", "Web Search Portals, Libraries, Archives, and Other Information Services", "", "519", 4},
	{"51913", "Internet Publishing and Broadcasting and Web Search Portals", "", "5191", 5},
	{"519130", "Internet Publishing and Broadcasting and Web Search Portals", "", "51913", 6},
	// Professional, Scientific, and Technical Services
	{"54", "Professional, Scientific, and Technical Services", "", "", 2},
	{"541", "Professional, Scientific, and Technical Services", "", "54", 3},
	// 5415: Computer Systems Design and Related Services - core federal IT NAICS
	{"5415", "Computer Systems Design and Related Services", "", "541", 4},
	{"54151", "Computer Systems Design and Related Services", "", "5415", 5},
	{"541511", "Custom Computer Programming Services", "", "54151", 6},
	{"541512", "Computer Systems Design Services", "", "54151", 6},
	{"541513", "Computer Facilities Management Services", "", "54151", 6},
	{"541519", "Other Computer Services", "", "54151", 6},
	// 5417: Scientific Research and Development Services
	{"5417", "Scientific Research and Development Services", "", "541", 4},
	{"54171", "Research and Development in the Physical, Engineering, and Life Sciences", "", "5417", 5},
	{"541713", "Research and Development in Nanotechnology", "", "54171", 6},
	{"541714", "Research and Development in Biotechnology (except Nanobiotechnology)", "", "54171", 6},
	{"541715", "Research and Development in the Physical, Engineering, and Life Sciences (except Nanotechnology and Biotechnology)", "", "54171", 6},
	{"54172", "Research and Development in the Social Sciences and Humanities", "", "5417", 5},
	{"541720", "Research and Development in the Social Sciences and Humanities", "", "54172", 6},
	// 5413: Architectural, Engineering, and Related Services (often IT mod)
	{"5413", "Architectural, Engineering, and Related Services", "", "541", 4},
	{"54133", "Engineering Services", "", "5413", 5},
	{"541330", "Engineering Services", "", "54133", 6},
	// 5416: Management, Scientific, and Technical Consulting Services
	{"5416", "Management, Scientific, and Technical Consulting Services", "", "541", 4},
	{"54161", "Management Consulting Services", "", "5416", 5},
	{"541611", "Administrative Management and General Management Consulting Services", "", "54161", 6},
	{"541612", "Human Resources Consulting Services", "", "54161", 6},
	{"541618", "Other Management Consulting Services", "", "54161", 6},
	{"54169", "Other Scientific and Technical Consulting Services", "", "5416", 5},
	{"541690", "Other Scientific and Technical Consulting Services", "", "54169", 6},
	// 5418: Advertising, Public Relations, and Related Services (occasionally IT)
	{"5418", "Advertising, Public Relations, and Related Services", "", "541", 4},
	// 5419: Other Professional, Scientific, and Technical Services
	{"5419", "Other Professional, Scientific, and Technical Services", "", "541", 4},
	// 5414: Specialized Design Services (UX/UI in fed IT mod)
	{"5414", "Specialized Design Services", "", "541", 4},
	{"54143", "Graphic Design Services", "", "5414", 5},
	{"541430", "Graphic Design Services", "", "54143", 6},
	// 5612: Facilities Support Services (occasionally IT ops)
	{"56", "Administrative and Support and Waste Management and Remediation Services", "", "", 2},
	{"561", "Administrative and Support Services", "", "56", 3},
	{"5612", "Facilities Support Services", "", "561", 4},
	{"561210", "Facilities Support Services", "", "5612", 6},
	{"5614", "Business Support Services", "", "561", 4},
	{"56142", "Telephone Call Centers", "", "5614", 5},
	{"561421", "Telephone Answering Services", "", "56142", 6},
	{"561422", "Telemarketing Bureaus and Other Contact Centers", "", "56142", 6},
	// 8112: Electronic and Precision Equipment Repair and Maintenance
	{"811", "Repair and Maintenance", "", "", 3},
	{"8112", "Electronic and Precision Equipment Repair and Maintenance", "", "811", 4},
	{"81121", "Electronic and Precision Equipment Repair and Maintenance", "", "8112", 5},
	{"811210", "Electronic and Precision Equipment Repair and Maintenance", "", "81121", 6},
	// 3341: Computer and Peripheral Equipment Manufacturing
	{"33", "Manufacturing", "", "", 2},
	{"334", "Computer and Electronic Product Manufacturing", "", "33", 3},
	{"3341", "Computer and Peripheral Equipment Manufacturing", "", "334", 4},
	{"33411", "Computer and Peripheral Equipment Manufacturing", "", "3341", 5},
	{"334111", "Electronic Computer Manufacturing", "", "33411", 6},
	{"334112", "Computer Storage Device Manufacturing", "", "33411", 6},
	{"334118", "Computer Terminal and Other Computer Peripheral Equipment Manufacturing", "", "33411", 6},
	// 3342: Communications Equipment Manufacturing
	{"3342", "Communications Equipment Manufacturing", "", "334", 4},
	{"33421", "Telephone Apparatus Manufacturing", "", "3342", 5},
	{"334210", "Telephone Apparatus Manufacturing", "", "33421", 6},
	{"33422", "Radio and Television Broadcasting and Wireless Communications Equipment Manufacturing", "", "3342", 5},
	{"334220", "Radio and Television Broadcasting and Wireless Communications Equipment Manufacturing", "", "33422", 6},
	// 5511: Management of Companies (parent for some integrators)
	{"55", "Management of Companies and Enterprises", "", "", 2},
	{"551", "Management of Companies and Enterprises", "", "55", 3},
	{"5511", "Management of Companies and Enterprises", "", "551", 4},
	{"551114", "Corporate, Subsidiary, and Regional Managing Offices", "", "55111", 6},
}

// PSCSeeds covers the D-series IT services codes and the 70-series IT
// products codes most often used for federal IT contracts.
var PSCSeeds = []CodeSeed{
	// D-series: IT services
	{"D", "Information Technology Services", "Services", "", 0},
	{"D301", "IT and Telecom - Facility Operation and Maintenance", "Services", "", 0},
	{"D302", "IT and Telecom - Systems Development", "Services", "", 0},
	{"D303", "IT and Telecom - Data Center Services", "Services", "", 0},
	{"D304", "IT and Telecom - Telecommunications and Transmission", "Services", "", 0},
	{"D305", "IT and Telecom - Teleprocessing, Timeshare, and Cloud Computing", "Services", "", 0},
	{"D306", "IT and Telecom - Systems Analysis Services", "Services", "", 0},
	{"D307", "IT and Telecom - IT Strategy and Architecture", "Services", "", 0},
	{"D308", "IT and Telecom - Programming Services", "Services", "", 0},
	{"D309", "IT and Telecom - Information and Data Broadcasting/Distribution Services", "Services", "", 0},
	{"D310", "IT and Telecom - IT Backup and Security Services", "Services", "", 0},
	{"D311", "IT and Telecom - Data Conversion Services", "Services", "", 0},
	{"D312", "IT and Telecom - Optical Scanning Services", "Services", "", 0},
	{"D313", "IT and Telecom - Computer Aided Design/Computer Aided Manufacturing (CAD/CAM)", "Services", "", 0},
	{"D314", "IT and Telecom - System Acquisition Support", "Services", "", 0},
	{"D315", "IT and Telecom - Digitizing Services (eg, cartographic, geographic, and image)", "Services", "", 0},
	{"D316", "IT and Telecom - Telecommunications Network Management Services", "Services", "", 0},
	{"D317", "IT and Telecom - Web-based Subscription Services", "Services", "", 0},
	{"D318", "IT and Telecom - Integrated Hardware/Software/Services Solutions", "Services", "", 0},
	{"D319", "IT and Telecom - Annual Software Maintenance Service Plans", "Services", "", 0},
	{"D320", "IT and Telecom - Annual Hardware Maintenance Service Plans", "Services", "", 0},
	{"D321", "IT and Telecom - Help Desk", "Services", "", 0},
	{"D322", "IT and Telecom - Internet", "Services", "", 0},
	{"D324", "IT and Telecom - Business Continuity (other than Disaster Recovery)", "Services", "", 0},
	{"D325", "IT and Telecom - Data Storage", "Services", "", 0},
	{"D330", "IT and Telecom - Cyber Security and Data Backup", "Services", "", 0},
	{"D399", "IT and Telecom - Other IT and Telecommunications", "Services", "", 0},
	// 70-series: IT products
	{"70", "Information Technology Equipment", "Products", "", 0},
	{"7010", "ADPE System Configuration", "Products", "", 0},
	{"7021", "ADP Central Processing Unit (CPU, Computer), Digital", "Products", "", 0},
	{"7022", "ADP Central Processing Unit (CPU, Computer), Analog", "Products", "", 0},
	{"7025", "ADP Input/Output and Storage Devices", "Products", "", 0},
	{"7030", "Information Technology Software", "Products", "", 0},
	{"7035", "ADP Support Equipment", "Products", "", 0},
	{"7045", "ADP Supplies", "Products", "", 0},
	{"7050", "ADP Components", "Products", "", 0},
	// Telecommunications products
	{"58", "Communications, Detection, and Coherent Radiation Equipment", "Products", "", 0},
	{"5805", "Telephone and Telegraph Equipment", "Products", "", 0},
	{"5810", "Communications Security Equipment and Components", "Products", "", 0},
	{"5811", "Other Cryptologic Equipment and Components", "Products", "", 0},
	{"5820", "Radio and Television Communication Equipment, Except Airborne", "Products", "", 0},
	{"5821", "Radio and Television Communication Equipment, Airborne", "Products", "", 0},
	// R&D PSC codes (engineering & IT modernization)
	{"AR", "Defense Systems R&D", "R&D", "", 0},
	{"AC", "Information and Communications R&D", "R&D", "", 0},
}

// CountSeeds returns (naics, psc) seed counts. Used by tests and `doctor`.
func CountSeeds() (int, int) { return len(NAICSSeeds), len(PSCSeeds) }
