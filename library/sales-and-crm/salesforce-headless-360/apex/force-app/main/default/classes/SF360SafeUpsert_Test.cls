@IsTest
private class SF360SafeUpsert_Test {
    @IsTest
    static void newIdlessRecordIsCreated() {
        callUpsert('{"sobject":"Contact","external_id_field":"Email","record":{"LastName":"SF360 Upsert","Email":"sf360-upsert@example.com"}}');

        System.assertEquals(200, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('"success":true'), body);
        System.assert(body.contains('"created":true'), body);
        System.assert(body.contains('"id"'), body);
    }

    @IsTest
    static void repeatExternalIdUpdatesExistingRecord() {
        callUpsert('{"sobject":"Contact","external_id_field":"Email","record":{"LastName":"SF360 First","Email":"sf360-repeat@example.com"}}');
        String firstBody = RestContext.response.responseBody.toString();
        System.assert(firstBody.contains('"created":true'), firstBody);

        callUpsert('{"sobject":"Contact","external_id_field":"Email","record":{"LastName":"SF360 Second","Email":"sf360-repeat@example.com"}}');

        System.assertEquals(200, RestContext.response.statusCode);
        String secondBody = RestContext.response.responseBody.toString();
        System.assert(secondBody.contains('"created":false'), secondBody);
        System.assertEquals(1, [SELECT count() FROM Contact WHERE Email = 'sf360-repeat@example.com']);
    }

    @IsTest
    static void strippedFieldsAreReported() {
        callUpsert('{"sobject":"Contact","external_id_field":"Email","record":{"LastName":"SF360 FLS","Email":"sf360-fls@example.com","CreatedDate":"2026-04-22T15:00:00.000Z"}}');

        System.assertEquals(200, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('"success":true'), body);
        System.assert(body.contains('CreatedDate'), body);
    }

    @IsTest
    static void malformedBodyReturnsError() {
        callUpsert('{"sobject":"Contact","record":{"LastName":"Missing external id"}}');

        System.assertEquals(400, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('MALFORMED_REQUEST'), body);
    }

    private static void callUpsert(String jsonBody) {
        RestRequest req = new RestRequest();
        req.requestUri = '/services/apexrest/sf360/v1/safeUpsert';
        req.httpMethod = 'POST';
        req.requestBody = Blob.valueOf(jsonBody);
        RestContext.request = req;
        RestContext.response = new RestResponse();

        SF360SafeUpsert.post();
    }
}
