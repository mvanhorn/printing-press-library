package client

import (
	"strings"
	"testing"
)

// Fixture: realistic Tableau PAT sign-in response (no network).
const signInFixture = `<?xml version='1.0' encoding='UTF-8'?>
<tsResponse xmlns="http://tableau.com/api" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://tableau.com/api https://help.tableau.com/samples/en-us/rest_api/ts-api_3_21.xsd">
  <credentials token="HvZMqFFfQQmOM4L-AZNIQA|5fI6T54OPK1Gn1p4w0RtHv6EkojWRTwq|a946d998-2ead-4894-bb50-1054a91dcab3" estimatedTimeToExpiration="14:00:00">
    <site id="9a8b7c6d-5e4f-3a2b-1c0d-9e8f7a6b5c4d" contentUrl="MarketingSite"/>
    <user id="d5704762-603a-42b2-a9a7-f9cd2b2c4253"/>
  </credentials>
</tsResponse>`

const signInErrorFixture = `<?xml version='1.0' encoding='UTF-8'?>
<tsResponse>
  <error code="401001">
    <summary>Signin Error</summary>
    <detail>Error signing in to Tableau Server (errorCode=16)</detail>
  </error>
</tsResponse>`

const projectsFixture = `<?xml version='1.0' encoding='UTF-8'?>
<tsResponse>
  <pagination pageNumber="1" pageSize="100" totalAvailable="2"/>
  <projects>
    <project id="1f2f3e4e5-d6d7-c8c9-b0b1-a2a3f4f5e6e" name="Default" description="The default project"/>
    <project id="972bcf8c-d7ce-11e3-ac00-0fa50cfedda9" name="Finance" description="Finance workbooks" parentProjectId="1f2f3e4e5-d6d7-c8c9-b0b1-a2a3f4f5e6e"/>
  </projects>
</tsResponse>`

const workbooksFixture = `<?xml version='1.0' encoding='UTF-8'?>
<tsResponse>
  <pagination pageNumber="1" pageSize="100" totalAvailable="1"/>
  <workbooks>
    <workbook id="2fbe87c9-a7d8-45bf-b2b3-877a26ec9af5" name="Superstore" contentUrl="Superstore" size="1024" createdAt="2024-01-15T12:00:00Z" updatedAt="2024-06-01T09:30:00Z" webpageUrl="https://example.online.tableau.com/#/site/MarketingSite/workbooks/123">
      <project id="972bcf8c-d7ce-11e3-ac00-0fa50cfedda9" name="Finance"/>
      <owner id="cdfe8548-84c8-418e-9b33-2c0728b2398a"/>
    </workbook>
  </workbooks>
</tsResponse>`

const publishFixture = `<?xml version='1.0' encoding='UTF-8'?>
<tsResponse>
  <workbook id="1a1b1c1d2-e2f2-a2b3-c3d3-e3f4a4b4c4d" name="Postal-rates" contentUrl="Postal-rates">
    <project id="1f2f3e4e5-d6d7-c8c9-b0b1-a2a3f4f5e6e" name="default"/>
    <owner id="a2dd5d79-4a63-40a6-9934-f94090ccb653"/>
  </workbook>
</tsResponse>`

const sitesFixture = `<?xml version='1.0' encoding='UTF-8'?>
<tsResponse>
  <pagination pageNumber="1" pageSize="100" totalAvailable="2"/>
  <sites>
    <site id="9a8b7c6d-5e4f-3a2b-1c0d-9e8f7a6b5c4d" name="Marketing" contentUrl="MarketingSite" state="Active"/>
    <site id="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" name="Default" contentUrl="" state="Active"/>
  </sites>
</tsResponse>`

const fileUploadFixture = `<?xml version='1.0' encoding='UTF-8'?>
<tsResponse>
  <fileUpload uploadSessionId="13253:6744F321974F4E8B8EC1424A3D56E0EA-0:0" fileSize="0"/>
</tsResponse>`

func TestParseSignInResponse(t *testing.T) {
	cred, err := ParseSignInResponse(strings.NewReader(signInFixture))
	if err != nil {
		t.Fatalf("ParseSignInResponse: %v", err)
	}
	if cred.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if !strings.Contains(cred.Token, "HvZMqFFfQQmOM4L") {
		t.Errorf("token = %q", cred.Token)
	}
	if cred.SiteID != "9a8b7c6d-5e4f-3a2b-1c0d-9e8f7a6b5c4d" {
		t.Errorf("SiteID = %q", cred.SiteID)
	}
	if cred.ContentURL != "MarketingSite" {
		t.Errorf("ContentURL = %q", cred.ContentURL)
	}
	if cred.UserID != "d5704762-603a-42b2-a9a7-f9cd2b2c4253" {
		t.Errorf("UserID = %q", cred.UserID)
	}
}

func TestParseSignInResponse_Error(t *testing.T) {
	_, err := ParseSignInResponse(strings.NewReader(signInErrorFixture))
	if err == nil {
		t.Fatal("expected error for sign-in error fixture")
	}
	if !strings.Contains(err.Error(), "401001") {
		t.Errorf("error should include code: %v", err)
	}
	if !strings.Contains(err.Error(), "Signin Error") {
		t.Errorf("error should include summary: %v", err)
	}
}

func TestParseSignInResponse_MissingToken(t *testing.T) {
	_, err := ParseSignInResponse(strings.NewReader(`<tsResponse><credentials token=""><site id="x"/></credentials></tsResponse>`))
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestParseProjectsResponse(t *testing.T) {
	projects, pag, err := ParseProjectsResponse(strings.NewReader(projectsFixture))
	if err != nil {
		t.Fatalf("ParseProjectsResponse: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(projects))
	}
	if projects[0].Name != "Default" {
		t.Errorf("projects[0].Name = %q", projects[0].Name)
	}
	if projects[1].ParentID != "1f2f3e4e5-d6d7-c8c9-b0b1-a2a3f4f5e6e" {
		t.Errorf("projects[1].ParentID = %q", projects[1].ParentID)
	}
	if pag == nil || pag.TotalAvailable != "2" {
		t.Errorf("pagination = %+v", pag)
	}
}

func TestParseWorkbooksResponse(t *testing.T) {
	workbooks, _, err := ParseWorkbooksResponse(strings.NewReader(workbooksFixture))
	if err != nil {
		t.Fatalf("ParseWorkbooksResponse: %v", err)
	}
	if len(workbooks) != 1 {
		t.Fatalf("len = %d, want 1", len(workbooks))
	}
	w := workbooks[0]
	if w.ID != "2fbe87c9-a7d8-45bf-b2b3-877a26ec9af5" {
		t.Errorf("ID = %q", w.ID)
	}
	if w.Name != "Superstore" {
		t.Errorf("Name = %q", w.Name)
	}
	if w.ProjectID != "972bcf8c-d7ce-11e3-ac00-0fa50cfedda9" {
		t.Errorf("ProjectID = %q", w.ProjectID)
	}
	if w.ProjectName != "Finance" {
		t.Errorf("ProjectName = %q", w.ProjectName)
	}
	if w.OwnerID == "" {
		t.Error("expected OwnerID")
	}
}

func TestParsePublishResponse(t *testing.T) {
	res, err := ParsePublishResponse(strings.NewReader(publishFixture))
	if err != nil {
		t.Fatalf("ParsePublishResponse: %v", err)
	}
	if res.ID != "1a1b1c1d2-e2f2-a2b3-c3d3-e3f4a4b4c4d" {
		t.Errorf("ID = %q", res.ID)
	}
	if res.Name != "Postal-rates" {
		t.Errorf("Name = %q", res.Name)
	}
	if res.ProjectID != "1f2f3e4e5-d6d7-c8c9-b0b1-a2a3f4f5e6e" {
		t.Errorf("ProjectID = %q", res.ProjectID)
	}
}

func TestParseSitesResponse(t *testing.T) {
	sites, pag, err := ParseSitesResponse(strings.NewReader(sitesFixture))
	if err != nil {
		t.Fatalf("ParseSitesResponse: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("len = %d, want 2", len(sites))
	}
	if sites[0].ContentURL != "MarketingSite" {
		t.Errorf("sites[0].ContentURL = %q", sites[0].ContentURL)
	}
	if sites[1].ContentURL != "" {
		t.Errorf("default site ContentURL should be empty, got %q", sites[1].ContentURL)
	}
	if pag == nil || pag.TotalAvailable != "2" {
		t.Errorf("pagination = %+v", pag)
	}
}

func TestParseFileUploadResponse(t *testing.T) {
	id, err := ParseFileUploadResponse(strings.NewReader(fileUploadFixture))
	if err != nil {
		t.Fatalf("ParseFileUploadResponse: %v", err)
	}
	if id != "13253:6744F321974F4E8B8EC1424A3D56E0EA-0:0" {
		t.Errorf("session id = %q", id)
	}
}

func TestBuildSignInPATRequest(t *testing.T) {
	xml := BuildSignInPATRequest("my-pat", "secret:value==", "MySite")
	if !strings.Contains(xml, `personalAccessTokenName="my-pat"`) {
		t.Errorf("missing name attr: %s", xml)
	}
	if !strings.Contains(xml, `personalAccessTokenSecret="secret:value=="`) {
		t.Errorf("missing secret attr: %s", xml)
	}
	if !strings.Contains(xml, `contentUrl="MySite"`) {
		t.Errorf("missing site: %s", xml)
	}
	// Special characters must be escaped — never raw injection.
	xml2 := BuildSignInPATRequest(`a"b`, `c&d`, `<e>`)
	if strings.Contains(xml2, `a"b`) {
		t.Error("quote in name should be escaped")
	}
	if !strings.Contains(xml2, "c&amp;d") {
		t.Errorf("ampersand should be escaped: %s", xml2)
	}
}

func TestBuildPublishWorkbookPayload(t *testing.T) {
	xml := BuildPublishWorkbookPayload("My Workbook", "proj-123")
	if !strings.Contains(xml, `name="My Workbook"`) {
		t.Errorf("missing name: %s", xml)
	}
	if !strings.Contains(xml, `id="proj-123"`) {
		t.Errorf("missing project: %s", xml)
	}
}

func TestBuildSignInPATRequest_DefaultSite(t *testing.T) {
	xml := BuildSignInPATRequest("n", "s", "")
	if !strings.Contains(xml, `contentUrl=""`) {
		t.Errorf("default site should have empty contentUrl: %s", xml)
	}
}
