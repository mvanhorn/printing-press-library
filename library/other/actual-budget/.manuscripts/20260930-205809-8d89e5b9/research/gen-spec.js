const swaggerJsdoc = require('swagger-jsdoc');
const pkg = require('./package.json');
const spec = swaggerJsdoc({definition:{openapi:'3.1.0',info:{title:'Actual HTTP Api',version:pkg.version},
 components:{securitySchemes:{apiKey:{type:'apiKey',name:'x-api-key',in:'header'}}},
 servers:[{url:'http://localhost:5007/v1'}]}, apis:['./src/v1/routes/*.js']});
process.stdout.write(JSON.stringify(spec,null,2));
