@IsTest
private class SF360SafeRead_Test {
    @IsTest
    static void happyPathOmitsUnavailableFieldsByUserMode() {
        Account acct = new Account(Name = 'SF360 Test');
        insert acct;

        RestRequest req = new RestRequest();
        req.requestUri = '/services/apexrest/sf360/v1/safeRead';
        req.httpMethod = 'POST';
        req.requestBody = Blob.valueOf('{"soql":"SELECT Id, Name FROM Account WHERE Id = \'' + acct.Id + '\'"}');
        RestContext.request = req;
        RestContext.response = new RestResponse();

        SF360SafeRead.post();

        System.assertEquals(200, RestContext.response.statusCode);
        String body = RestContext.response.responseBody.toString();
        System.assert(body.contains('"ok":true'), body);
        System.assert(body.contains(acct.Id), body);
    }

    @IsTest
    static void badSoqlReturnsErrorPayload() {
        RestRequest req = new RestRequest();
        req.requestUri = '/services/apexrest/sf360/v1/safeRead';
        req.httpMethod = 'POST';
        req.requestBody = Blob.valueOf('{"soql":"SELECT NoSuchField__c FROM Account"}');
        RestContext.request = req;
        RestContext.response = new RestResponse();

        SF360SafeRead.post();

        System.assertEquals(400, RestContext.response.statusCode);
        System.assert(RestContext.response.responseBody.toString().contains('SF360.D9.SOQL_ERROR'));
    }

    @IsTest
    static void nonSelectStatementIsRefused() {
        RestRequest req = new RestRequest();
        req.requestUri = '/services/apexrest/sf360/v1/safeRead';
        req.httpMethod = 'POST';
        req.requestBody = Blob.valueOf('{"soql":"DELETE FROM Account"}');
        RestContext.request = req;
        RestContext.response = new RestResponse();

        SF360SafeRead.post();

        System.assertEquals(400, RestContext.response.statusCode);
        System.assert(RestContext.response.responseBody.toString().contains('SF360.D9.SECURITY_ERROR'));
    }

    @IsTest
    static void explicitUserModeIsAccepted() {
        RestRequest req = new RestRequest();
        req.requestUri = '/services/apexrest/sf360/v1/safeRead';
        req.httpMethod = 'POST';
        req.requestBody = Blob.valueOf('{"soql":"SELECT Id FROM Account WITH USER_MODE"}');
        RestContext.request = req;
        RestContext.response = new RestResponse();

        SF360SafeRead.post();

        System.assertEquals(200, RestContext.response.statusCode);
    }
}
