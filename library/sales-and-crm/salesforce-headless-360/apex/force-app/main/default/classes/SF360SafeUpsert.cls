@RestResource(urlMapping='/sf360/v1/safeUpsert')
global with sharing class SF360SafeUpsert {
    global class RequestBody {
        public String sobject;
        public String external_id_field;
        public Map<String, Object> record;
    }

    global class ErrorBody {
        public String code;
        public String message;
        public Integer http_status;
        public Map<String, Object> data;
    }

    global class ResponseBody {
        public Boolean success;
        public String id;
        public Boolean created;
        public String last_modified_date;
        public List<String> fls_filtered;
        public ErrorBody error;
    }

    private class MalformedRequestException extends Exception {}
    private class RefusedSystemSObjectException extends Exception {}

    @HttpPost
    global static void post() {
        RestResponse res = RestContext.response;
        res.addHeader('Content-Type', 'application/json');
        try {
            RequestBody input = (RequestBody) JSON.deserialize(RestContext.request.requestBody.toString(), RequestBody.class);
            validateInput(input);
            Schema.SObjectType sobjectType = validateSObject(input.sobject);
            Schema.SObjectField externalIdField = validateExternalIdField(sobjectType, input.external_id_field);
            SObject sobj = buildRecord(sobjectType, input.record);

            SObjectAccessDecision decision = Security.stripInaccessible(
                AccessType.UPSERTABLE,
                new List<SObject>{ sobj },
                false
            );
            SObject safeRecord = decision.getRecords()[0];
            Set<String> stripped = removedFieldsFor(decision, sobjectType.getDescribe().getName());
            if (input.record.containsKey('Id')) {
                safeRecord.put('Id', input.record.get('Id'));
                stripped.remove('Id');
            }

            Database.UpsertResult result = Database.upsert(safeRecord, externalIdField, AccessLevel.USER_MODE);

            ResponseBody body = new ResponseBody();
            body.success = true;
            body.id = String.valueOf(result.getId() == null ? safeRecord.get('Id') : result.getId());
            body.created = result.isCreated();
            body.last_modified_date = lastModifiedDate(sobjectType, body.id);
            body.fls_filtered = sortedList(stripped);
            res.statusCode = 200;
            res.responseBody = Blob.valueOf(JSON.serialize(body));
        } catch (MalformedRequestException e) {
            writeError(res, 400, 'MALFORMED_REQUEST', e.getMessage(), new Map<String, Object>());
        } catch (RefusedSystemSObjectException e) {
            writeError(res, 400, 'REFUSED_SYSTEM_SOBJECT', e.getMessage(), new Map<String, Object>());
        } catch (DmlException e) {
            writeDmlError(res, e);
        } catch (Exception e) {
            writeError(res, 500, 'SERVER_ERROR', e.getMessage(), new Map<String, Object>());
        }
    }

    private static void validateInput(RequestBody input) {
        if (input == null) {
            throw new MalformedRequestException('Request body is required');
        }
        if (String.isBlank(input.sobject)) {
            throw new MalformedRequestException('sobject is required');
        }
        if (String.isBlank(input.external_id_field)) {
            throw new MalformedRequestException('external_id_field is required');
        }
        if (input.record == null || input.record.isEmpty()) {
            throw new MalformedRequestException('record is required');
        }
        if (!input.record.containsKey(input.external_id_field) && input.external_id_field != 'Id') {
            throw new MalformedRequestException('record must include external_id_field');
        }
    }

    private static Schema.SObjectType validateSObject(String apiName) {
        String normalized = apiName == null ? null : apiName.trim();
        if (String.isBlank(normalized)) {
            throw new MalformedRequestException('sobject is required');
        }
        Schema.SObjectType sobjectType = Schema.getGlobalDescribe().get(normalized);
        if (sobjectType == null) {
            throw new MalformedRequestException('Unknown sobject: ' + normalized);
        }
        Schema.DescribeSObjectResult describe = sobjectType.getDescribe();
        if (isRefusedSystemSObject(describe.getName())) {
            throw new RefusedSystemSObjectException('Writes to ' + describe.getName() + ' are refused by SF360');
        }
        return sobjectType;
    }

    private static Boolean isRefusedSystemSObject(String apiName) {
        Set<String> refused = new Set<String>{
            'ApexClass',
            'ApexTrigger',
            'AsyncApexJob',
            'AuthSession',
            'CronTrigger',
            'FieldPermissions',
            'Group',
            'GroupMember',
            'LoginHistory',
            'ObjectPermissions',
            'Organization',
            'PermissionSet',
            'PermissionSetAssignment',
            'Profile',
            'QueueSObject',
            'SetupAuditTrail',
            'SetupEntityAccess',
            'User',
            'UserRole'
        };
        return refused.contains(apiName);
    }

    private static Schema.SObjectField validateExternalIdField(Schema.SObjectType sobjectType, String fieldName) {
        Map<String, Schema.SObjectField> fields = sobjectType.getDescribe().fields.getMap();
        if (!fields.containsKey(fieldName)) {
            throw new MalformedRequestException('Unknown external_id_field on ' + sobjectType.getDescribe().getName() + ': ' + fieldName);
        }
        Schema.DescribeFieldResult describe = fields.get(fieldName).getDescribe();
        if (fieldName != 'Id' && !describe.isExternalId() && !describe.isIdLookup()) {
            throw new MalformedRequestException(fieldName + ' is not an external id or idLookup field');
        }
        return fields.get(fieldName);
    }

    private static SObject buildRecord(Schema.SObjectType sobjectType, Map<String, Object> values) {
        SObject sobj = sobjectType.newSObject();
        Map<String, Schema.SObjectField> fields = sobjectType.getDescribe().fields.getMap();
        for (String fieldName : values.keySet()) {
            if (!fields.containsKey(fieldName)) {
                throw new MalformedRequestException('Unknown field on ' + sobjectType.getDescribe().getName() + ': ' + fieldName);
            }
            Schema.DescribeFieldResult fieldDescribe = fields.get(fieldName).getDescribe();
            sobj.put(fieldName, coerceValue(values.get(fieldName), fieldDescribe));
        }
        return sobj;
    }

    private static Object coerceValue(Object value, Schema.DescribeFieldResult fieldDescribe) {
        if (value == null || !(value instanceof String)) {
            return value;
        }
        String text = (String) value;
        Schema.DisplayType fieldType = fieldDescribe.getType();
        if (fieldType == Schema.DisplayType.Date) {
            return Date.valueOf(text);
        }
        if (fieldType == Schema.DisplayType.DateTime) {
            return (Datetime) JSON.deserialize('"' + text + '"', Datetime.class);
        }
        if (fieldType == Schema.DisplayType.Boolean) {
            return Boolean.valueOf(text);
        }
        if (fieldType == Schema.DisplayType.Integer) {
            return Integer.valueOf(text);
        }
        if (fieldType == Schema.DisplayType.Currency || fieldType == Schema.DisplayType.Double ||
            fieldType == Schema.DisplayType.Percent) {
            return Decimal.valueOf(text);
        }
        return value;
    }

    private static Set<String> removedFieldsFor(SObjectAccessDecision decision, String sobjectName) {
        Map<String, Set<String>> removed = decision.getRemovedFields();
        if (removed.containsKey(sobjectName)) {
            return new Set<String>(removed.get(sobjectName));
        }
        return new Set<String>();
    }

    private static String lastModifiedDate(Schema.SObjectType sobjectType, String recordId) {
        if (String.isBlank(recordId)) {
            return null;
        }
        String query = 'SELECT LastModifiedDate FROM ' + sobjectType.getDescribe().getName() + ' WHERE Id = :recordId';
        SObject row = Database.query(query);
        Datetime value = (Datetime) row.get('LastModifiedDate');
        return value == null ? null : value.formatGmt('yyyy-MM-dd\'T\'HH:mm:ss.SSS\'Z\'');
    }

    private static List<String> sortedList(Set<String> values) {
        List<String> out = new List<String>();
        out.addAll(values);
        out.sort();
        return out;
    }

    private static void writeDmlError(RestResponse res, DmlException e) {
        Map<String, Object> data = new Map<String, Object>();
        if (e.getNumDml() > 0) {
            data.put('status_code', String.valueOf(e.getDmlStatusCode(0)));
            data.put('fields', e.getDmlFieldNames(0));
            if (e.getDmlStatusCode(0) == StatusCode.FIELD_CUSTOM_VALIDATION_EXCEPTION) {
                data.put('rule_name', String.valueOf(e.getDmlStatusCode(0)));
            }
            writeError(res, 400, dmlCode(e.getDmlStatusCode(0)), e.getDmlMessage(0), data);
            return;
        }
        writeError(res, 400, 'DML_REJECTED', e.getMessage(), data);
    }

    private static String dmlCode(StatusCode statusCode) {
        if (statusCode == StatusCode.FIELD_CUSTOM_VALIDATION_EXCEPTION) {
            return 'VALIDATION_RULE_REJECTED';
        }
        if (statusCode == StatusCode.REQUIRED_FIELD_MISSING) {
            return 'REQUIRED_FIELD_MISSING';
        }
        if (statusCode == StatusCode.INSUFFICIENT_ACCESS_OR_READONLY ||
            statusCode == StatusCode.INVALID_FIELD_FOR_INSERT_UPDATE) {
            return 'FLS_WRITE_DENIED';
        }
        return String.valueOf(statusCode);
    }

    private static void writeError(RestResponse res, Integer status, String code, String message, Map<String, Object> data) {
        ResponseBody body = new ResponseBody();
        body.success = false;
        body.fls_filtered = new List<String>();
        ErrorBody err = new ErrorBody();
        err.code = code;
        err.message = message;
        err.http_status = status;
        err.data = data;
        body.error = err;
        res.statusCode = status;
        res.responseBody = Blob.valueOf(JSON.serialize(body));
    }
}
