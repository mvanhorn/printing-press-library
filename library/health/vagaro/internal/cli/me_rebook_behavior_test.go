// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAppointments_flat(t *testing.T) {
	data := json.RawMessage(`{"status":"success","data":[
		{"appointmentId":9001,"businessId":93458,"businessName":"Central Barber","vagaroURL":"centralbarber","serviceId":34098477,"serviceName":"Skin Fade","serviceProviderId":43931725,"serviceProviderFirstName":"Ronnel","serviceProviderLastName":"Getz","startDate":"2026-06-01"},
		{"appointmentId":9002,"businessId":55,"serviceId":66,"providerId":77}
	]}`)
	appts, err := parseAppointments(data)
	require.NoError(t, err)
	require.Len(t, appts, 2)

	assert.Equal(t, "9001", appts[0].AppointmentID)
	assert.Equal(t, "93458", appts[0].BusinessID)
	assert.Equal(t, "centralbarber", appts[0].BusinessSlug)
	assert.Equal(t, "Central Barber", appts[0].BusinessName)
	assert.Equal(t, "34098477", appts[0].ServiceID)
	assert.Equal(t, "Skin Fade", appts[0].ServiceName)
	assert.Equal(t, "43931725", appts[0].ProviderID)
	assert.Equal(t, "Ronnel Getz", appts[0].ProviderName)

	assert.Equal(t, "55", appts[1].BusinessID)
	assert.Equal(t, "77", appts[1].ProviderID)
}

func TestParseAppointments_nested(t *testing.T) {
	data := json.RawMessage(`{"data":[
		{"id":1,"business":{"id":93458,"name":"Central Barber"},"service":{"id":34098477,"name":"Skin Fade"},"serviceProvider":{"id":43931725,"name":"Ronnel Getz"}}
	]}`)
	appts, err := parseAppointments(data)
	require.NoError(t, err)
	require.Len(t, appts, 1)
	assert.Equal(t, "93458", appts[0].BusinessID)
	assert.Equal(t, "Skin Fade", appts[0].ServiceName)
	assert.Equal(t, "43931725", appts[0].ProviderID)
}

func TestParseAppointments_empty(t *testing.T) {
	appts, err := parseAppointments(json.RawMessage(`{"status":"success","data":[]}`))
	require.NoError(t, err)
	assert.Empty(t, appts)

	// Non-JSON (e.g. an auth HTML page) is an error, not a silent empty.
	_, err = parseAppointments(json.RawMessage(`<html>login</html>`))
	assert.Error(t, err)
}

func TestResolveRebookServiceAndProviderFallsBackFromAppointmentIDsToNames(t *testing.T) {
	services := []vagaro.ServiceRow{
		{ServiceID: 101, ServiceTitle: "Classic Cut"},
		{ServiceID: 202, ServiceTitle: "Skin Fade"},
	}
	providers := []vagaro.Provider{
		{ServiceProviderID: 301, Name: "Ronnel Getz"},
		{ServiceProviderID: 302, Name: "Alex Rivera"},
	}
	appt := rebookAppointment{
		BusinessSlug: "centralbarber",
		ServiceID:    "encrypted-service-id-from-history",
		ServiceName:  "Skin Fade",
		ProviderID:   "999999999", // numeric, but not the public availability provider id
		ProviderName: "Ronnel",
	}

	service, err := resolveRebookService(services, appt)
	require.NoError(t, err)
	assert.Equal(t, int64(202), service.ServiceID)

	providerID, providerName, err := resolveRebookProvider(providers, appt)
	require.NoError(t, err)
	assert.Equal(t, "301", providerID)
	assert.Equal(t, "Ronnel Getz", providerName)
}

func TestResolveRebookServiceAndProviderPreferMatchingPublicIDs(t *testing.T) {
	services := []vagaro.ServiceRow{{ServiceID: 202, ServiceTitle: "Skin Fade"}}
	providers := []vagaro.Provider{{ServiceProviderID: 301, Name: "Ronnel Getz"}}
	appt := rebookAppointment{ServiceID: "202", ServiceName: "Wrong Name", ProviderID: "301", ProviderName: "Wrong Provider"}

	service, err := resolveRebookService(services, appt)
	require.NoError(t, err)
	assert.Equal(t, int64(202), service.ServiceID)

	providerID, providerName, err := resolveRebookProvider(providers, appt)
	require.NoError(t, err)
	assert.Equal(t, "301", providerID)
	assert.Equal(t, "Ronnel Getz", providerName)
}

func TestResolveRebookServiceReportsActionableAmbiguity(t *testing.T) {
	services := []vagaro.ServiceRow{
		{ServiceID: 101, ServiceTitle: "Kids Skin Fade"},
		{ServiceID: 202, ServiceTitle: "Adult Skin Fade"},
	}
	appt := rebookAppointment{BusinessSlug: "centralbarber", ServiceID: "encrypted", ServiceName: "Skin Fade", ProviderName: "Ronnel"}

	_, err := resolveRebookService(services, appt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matched multiple services")
	diag := rebookDiagnostic(appt, err.Error())
	assert.Contains(t, diag, "vagaro-pp-cli business availability centralbarber")
	assert.Contains(t, diag, `--service "Skin Fade"`)
}

func TestSlugFromAppointmentURL(t *testing.T) {
	assert.Equal(t, "centralbarber", slugFromAppointmentURL("https://www.vagaro.com/centralbarber/book-now"))
	assert.Equal(t, "centralbarber", slugFromAppointmentURL("www.vagaro.com/centralbarber"))
	assert.Equal(t, "", slugFromAppointmentURL("https://centralbarber.com/contact"))
	assert.Equal(t, "", slugFromAppointmentURL(""))
}

func TestParseAppointmentsGenericURLDoesNotBecomeBusinessSlug(t *testing.T) {
	data := json.RawMessage(`{"data":[
		{"appointmentId":9001,"businessId":93458,"businessName":"Central Barber","url":"https://centralbarber.com/contact","serviceName":"Skin Fade"}
	]}`)
	appts, err := parseAppointments(data)
	require.NoError(t, err)
	require.Len(t, appts, 1)
	assert.Equal(t, "93458", appts[0].BusinessID)
	assert.Equal(t, "", appts[0].BusinessSlug)
}
