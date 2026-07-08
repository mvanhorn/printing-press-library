@IsTest
private class SF360SafeWrite_Test {
    @IsTest
    static void updateAccountReturnsSuccessEnvelope() {
        Account acct = new Account(Name = 'SF360 Before');
        insert acct;

        callWrite('{"operation":"update","sobject":"Account","record":{"Id":"' + acct.Id + '","Name":"SF360 After"},"fields_requested":["Name"]}');

        System.assertEquals(200, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('"success":true'), body);
        System.assert(body.contains(acct.Id), body);
        System.assert(body.contains('"last_modified_date"'), body);
    }

    @IsTest
    static void strippedFieldsAreReportedWithoutBlockingWrite() {
        Account acct = new Account(Name = 'SF360 FLS');
        insert acct;

        callWrite('{"operation":"update","sobject":"Account","record":{"Id":"' + acct.Id + '","Name":"SF360 FLS Updated","CreatedDate":"2026-04-22T15:00:00.000Z"},"fields_requested":["Name","CreatedDate"]}');

        System.assertEquals(200, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('"success":true'), body);
        System.assert(body.contains('CreatedDate'), body);
    }

    @IsTest
    static void requiredFieldMissingReturnsD9Envelope() {
        callWrite('{"operation":"insert","sobject":"Account","record":{"Description":"missing name"},"fields_requested":["Description"]}');

        System.assertEquals(400, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('"success":false'), body);
        System.assert(body.contains('REQUIRED_FIELD_MISSING'), body);
    }

    @IsTest
    static void systemSObjectIsRefused() {
        callWrite('{"operation":"update","sobject":"User","record":{"Id":"' + UserInfo.getUserId() + '","LastName":"Blocked"}}');

        System.assertEquals(400, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('REFUSED_SYSTEM_SOBJECT'), body);
    }

    @IsTest
    static void malformedBodyReturnsError() {
        callWrite('{"sobject":"Account","record":{"Name":"Missing operation"}}');

        System.assertEquals(400, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('MALFORMED_REQUEST'), body);
    }

    private static void callWrite(String jsonBody) {
        RestRequest req = new RestRequest();
        req.requestUri = '/services/apexrest/sf360/v1/safeWrite';
        req.httpMethod = 'POST';
        req.requestBody = Blob.valueOf(jsonBody);
        RestContext.request = req;
        RestContext.response = new RestResponse();

        Test.startTest();
        SF360SafeWrite.post();
        Test.stopTest();
    }
}
