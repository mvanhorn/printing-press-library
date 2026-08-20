package closedsignal

import "testing"

func TestCheckTabelogHTML(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		wantClosed bool
		wantTemp   bool
	}{
		{"empty", "", false, false},
		{"open page", "<html>delicious sushi</html>", false, false},
		{"permanent close kanji", "<title>店名 - 閉店</title>", true, false},
		{"business ended", "<div>営業終了しました</div>", true, false},
		{"on break temporary", "<div>現在お休み中</div>", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := CheckTabelogHTML(tt.html)
			if v.Closed != tt.wantClosed {
				t.Errorf("Closed=%v, want %v", v.Closed, tt.wantClosed)
			}
			if v.Temporary != tt.wantTemp {
				t.Errorf("Temporary=%v, want %v", v.Temporary, tt.wantTemp)
			}
		})
	}
}

func TestCheckNaverHTML(t *testing.T) {
	if !CheckNaverHTML("<div>폐업</div>").Closed {
		t.Error("naver 폐업 should be closed")
	}
	if CheckNaverHTML("").Closed {
		t.Error("empty should be open")
	}
}

func TestCheckLeFoodingHTML(t *testing.T) {
	if !CheckLeFoodingHTML("<p>Fermé définitivement</p>").Closed {
		t.Error("le fooding 'fermé définitivement' should be closed (case-insensitive)")
	}
	if CheckLeFoodingHTML("").Closed {
		t.Error("empty should be open")
	}
}

func TestCheckOSMTags(t *testing.T) {
	if !CheckOSMTags(map[string]string{"disused:amenity": "restaurant"}).Closed {
		t.Error("disused:amenity should be closed")
	}
	if !CheckOSMTags(map[string]string{"opening_hours": "closed"}).Closed {
		t.Error("opening_hours=closed should be closed")
	}
	if !CheckOSMTags(map[string]string{"opening_hours": "OFF"}).Closed {
		t.Error("opening_hours=off (case-insensitive) should be closed")
	}
	if CheckOSMTags(map[string]string{"amenity": "restaurant"}).Closed {
		t.Error("normal amenity should be open")
	}
	if CheckOSMTags(nil).Closed {
		t.Error("nil tags should be open")
	}
}

func TestCheckGoogleBusinessStatus(t *testing.T) {
	if !CheckGoogleBusinessStatus("CLOSED_PERMANENTLY").Closed {
		t.Error("CLOSED_PERMANENTLY should be closed")
	}
	if !CheckGoogleBusinessStatus("CLOSED_TEMPORARILY").Temporary {
		t.Error("CLOSED_TEMPORARILY should be Temporary")
	}
	if CheckGoogleBusinessStatus("OPERATIONAL").Closed {
		t.Error("OPERATIONAL should be open")
	}
	if CheckGoogleBusinessStatus("").Closed {
		t.Error("empty should be open")
	}
}

func TestCheckReviewText(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"empty", "", false},
		{"normal", "great food and atmosphere", false},
		{"permanently closed", "Sadly, permanently closed last month.", true},
		{"closed for good", "they closed for good in 2024", true},
		{"rip", "RIP, this place was great", true},
		{"now closed", "Visit it before it's now closed", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := CheckReviewText(tt.body, 12)
			if v.Closed != tt.want {
				t.Errorf("Closed=%v, want %v", v.Closed, tt.want)
			}
		})
	}
}

func TestCombine(t *testing.T) {
	if Combine(nil).Closed {
		t.Error("empty input should be Open")
	}
	if Combine([]Verdict{Open, Open}).Closed {
		t.Error("all-Open should be Open")
	}
	closed := Verdict{Closed: true, Source: "tabelog", Evidence: "閉店"}
	temp := Verdict{Temporary: true, Source: "google.business_status"}
	if !Combine([]Verdict{Open, temp, closed}).Closed {
		t.Error("Closed should beat Temporary")
	}
	got := Combine([]Verdict{Open, temp, Open})
	if !got.Temporary || got.Closed {
		t.Error("Temporary alone should win")
	}
}
