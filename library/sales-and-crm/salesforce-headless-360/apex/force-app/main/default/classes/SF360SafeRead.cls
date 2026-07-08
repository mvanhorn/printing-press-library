@RestResource(urlMapping='/sf360/v1/safeRead')
global with sharing class SF360SafeRead {
    global class RequestBody {
        public String soql;
        public Map<String, Object> params;
    }

    global class ErrorEnvelope {
        public String code;
        public String message;
        public String stage;
        public String hint;
    }

    global class ResponseBody {
        public Boolean ok;
        public List<SObject> records;
        public ErrorEnvelope error;
    }

    @HttpPost
    global static void post() {
        RestResponse res = RestContext.response;
        res.addHeader('Content-Type', 'application/json');
        try {
            RequestBody input = (RequestBody) JSON.deserialize(RestContext.request.requestBody.toString(), RequestBody.class);
            String soql = input == null ? null : input.soql;
            validateSelectOnly(soql);
            String userModeSOQL = withUserMode(soql);
            List<SObject> rows = Database.query(userModeSOQL);
            ResponseBody body = new ResponseBody();
            body.ok = true;
            body.records = rows;
            res.statusCode = 200;
            res.responseBody = Blob.valueOf(JSON.serialize(body));
        } catch (QueryException e) {
            writeError(res, 400, 'SOQL_ERROR', e.getMessage(), 'safe_read.query', 'Check field names, object access, and WITH USER_MODE support in this org.');
        } catch (SecurityException e) {
            writeError(res, 400, 'SECURITY_ERROR', e.getMessage(), 'safe_read.validate', 'Only SELECT statements are accepted.');
        } catch (Exception e) {
            writeError(res, 500, 'SERVER_ERROR', e.getMessage(), 'safe_read.execute', 'Deploy with an admin profile and retry.');
        }
    }

    private static void validateSelectOnly(String soql) {
        if (String.isBlank(soql)) {
            throw new SecurityException('SOQL is required');
        }
        String normalized = soql.trim().toLowerCase();
        if (!normalized.startsWith('select ')) {
            throw new SecurityException('Only SELECT SOQL is supported');
        }
        if (normalized.contains(';') || normalized.startsWith('find ') || normalized.startsWith('insert ') ||
            normalized.startsWith('update ') || normalized.startsWith('delete ') || normalized.startsWith('undelete ')) {
            throw new SecurityException('Only a single SELECT statement is supported');
        }
    }

    private static String withUserMode(String soql) {
        String trimmed = soql.trim();
        if (trimmed.toLowerCase().contains(' with user_mode')) {
            return trimmed;
        }
        return trimmed + ' WITH USER_MODE';
    }

    private static void writeError(RestResponse res, Integer status, String code, String message, String stage, String hint) {
        ResponseBody body = new ResponseBody();
        body.ok = false;
        ErrorEnvelope err = new ErrorEnvelope();
        err.code = 'SF360.D9.' + code;
        err.message = message;
        err.stage = stage;
        err.hint = hint;
        body.error = err;
        res.statusCode = status;
        res.responseBody = Blob.valueOf(JSON.serialize(body));
    }
}
