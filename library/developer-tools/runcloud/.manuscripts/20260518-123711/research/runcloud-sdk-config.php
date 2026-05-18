<?php

return [
    'domain'    => 'https://runcloud-dashboard.test/api/v3',
    'base'      => '/agency',
    'agency-api' => [
        // account
        'get-agency-account'    => ['GET', '/account', 'Get agency account details'],

        // clients
        'get-clients'               => ['GET', '/clients', 'Get all agency clients'],
        'create-client'             => ['POST', '/clients', 'Create agency client'],
        'get-client-details'                => ['GET', '/clients/%s', 'Get an agency client details'],
        'delete-client'             => ['DELETE', '/clients/%s', 'Delete agency client'],
        'update-client'             => ['PUT', '/clients/%s', 'Update agency client details'],
        'update-client-password'    => [
            'PUT',
            '/clients/%s/changepassword',
            'Change an agency client password acccess to client dashboard'
        ],
        'create-client-magic-link'  => [
            'POST',
            '/clients/%s/magiclink',
            'Add agency client magic link for quick access to client dashboard'
        ],
        'update-client-tag'         => ['PUT', '/clients/%s/tags', 'Update agency client tags'],

        // packages
        'get-server-packages'                   => ['GET', '/server-packages', 'Get all agency server packages'],
        'create-server-package'                 => ['POST', '/server-packages', 'Create agency server packages'],
        'get-server-package-details'                    => [
            'GET',
            '/server-packages/%s',
            'Get an agency server package details'
        ],
        'get-server-packages-upgrades'          => [
            'GET',
            '/server-packages/upgrades',
            'Get agency server packages available upgrade plans'
        ],
        'update-server-package'                 => [
            'PUT',
            '/server-packages/%s',
            'Update agency server package details'
        ],
        'delete-server-package'                 => ['DELETE', '/server-packages/%s', 'Delete agency server package'],
        'get-server-package-available-upgrades' => [
            'GET',
            '/server-packages/%s/available-upgrades',
            'Get current server package available upgrade plans'
        ],
        'get-server-package-client-servers'     => [
            'GET',
            '/server-packages/%s/client-servers',
            'Get current server package client servers'
        ],
        'duplicate-server-package'              => [
            'POST',
            '/server-packages/%s/duplicate',
            'Duplicate an agency server package'
        ],

        // teams
        'get-teams'                         => ['GET', '/teams', 'Get all agency teams'],
        'create-team'                       => ['POST', '/teams', 'Create agency team'],
        'get-team-details'                  => ['GET', '/teams/%s', 'Get agency team details'],
        'update-team'                       => ['PUT', '/teams/%s', 'Update agency team details'],
        'delete-team'                       => ['DELETE', '/teams/%s', 'Delete agency team'],
        'create-team-member-invitation'     => [
            'POST',
            '/teams/%s/members/invitations',
            'Send an agency team member invitation'
        ],
        'delete-team-member-invitation'     => [
            'DELETE',
            '/teams/%s/members/invitations/%s',
            'Cancel agency team member invitation'
        ],
        'get-team-member-details'           => ['GET', '/teams/%s/members/%s', 'Get agency team member details'],
        'delete-team-member'                => ['DELETE', '/teams/%s/members/%s', 'Remove agency team member'],
        'update-team-member'                => ['PUT', '/teams/%s/members/%s', 'Update agency team member details'],
        'transfer-team-member'              => [
            'PUT',
            '/teams/%s/members/%s/transfer',
            'Transfer agencu team member to another agency teams'
        ],
        'add-team-server-packages'          => [
            'POST',
            '/teams/%s/server-packages',
            'Import a server package into agency team'
        ],
        'delete-team-server-package'        => [
            'DELETE',
            '/teams/%s/server-packages/%s',
            'Remove server package from agency team'
        ],
        'add-team-servers'                  => ['POST', '/teams/%s/servers', 'Add server into agency team'],
        'delete-team-server'                => ['DELETE', '/teams/%s/servers/%s', 'Remove server from agency team'],

        // client servers
        'create-client-server'                          => ['POST', '/client-servers'],
        'get-client-servers'                            => ['GET', '/client-servers'],
        'get-client-server-details'                     => ['GET', '/client-servers/%s'],
        'add-ip-address-to-client-server'               => ['POST', '/client-servers/%/add-ipaddress'],
        'assign-server-to-client-server'                => ['POST', '/client-servers/%s/assign-server'],
        'change-client-server-client'                   => ['POST', '/client-servers/%s/change-client'],
        'change-client-server-dashboard-and-features'   => [
            'POST',
            '/client-servers/%s/change-client-dashboard-and-features'
        ],
        'delete-client-server'                          => ['DELETE', '/client-servers/%s'],
        'rebuild-client-server'                         => ['POST', '/client-servers/%s/rebuild-server'],
        'resend-client-server-webhook'                  => ['POST', '/client-servers/%s/resend-webhook'],
        'suspend-client-server'                         => ['POST', '/client-servers/%s/suspend'],
        'unsuspend-client-server'                       => ['POST', '/client-servers/%s/unsuspend'],
        'upgrade-client-server-package'                 => [
            'POST',
            '/client-servers/%s/upgrade-server-package'
        ]

    ],
    'core-api'  => [
        // static data
        'get-database-collations'           => ['GET', '/static/databases/collations'],
        'get-timezones'                     => ['GET', '/static/timezones'],
        'get-available-script-installer'    => ['GET', '/static/webapps/installers'],
        'get-available-ssl-protocols'       => ['GET', '/static/ssl/protocols'],

        // 3rd party api key
        'create-api-key'        => ['POST', '/settings/externalapi'],
        'get-api-keys'          => ['GET', '/settings/externalapi'],
        'get-api-key-details'   => ['GET', '/settings/externalapi/%s'],
        'update-api-key'        => ['PATCH', '/settings/externalapi/%s'],
        'delete-api-key'        => ['DELETE', '/settings/externalapi/%s'],

        // servers
        'create-server'                             => ['POST', '/servers'],
        'get-servers'                               => ['GET', '/servers'],
        'get-shared-servers'                        => ['GET', '/servers/shared'],
        'get-server-details'                        => ['GET', '/servers/%s'],
        'get-server-installation-script'            => ['GET', '/servers/%s/installationscript'],
        'get-server-stats'                          => ['GET', '/servers/%s/stats'],
        'get-server-hardware-info'                  => ['GET', '/servers/%s/hardwareinfo'],
        'get-available-server-php-version'          => ['GET', '/servers/%s/php/version'],
        'update-server-php-cli-version'             => ['PATCH', '/servers/%s/php/cli'],
        'update-server-metadata'                    => ['PATCH', '/servers/%s/settings/meta'],
        'get-server-ssh-configuration'              => ['GET', '/servers/%s/settings/ssh'],
        'update-server-ssh-configuration'           => ['PATCH', '/servers/%s/settings/ssh'],
        'update-server-software-update-settings'    => ['PATCH', '/servers/%s/settings/autoupdate'],
        'delete-server'                             => ['DELETE', '/servers/%s'],

        // health
        'get-server-lastest-health-data'    => ['GET', '/servers/%s/health/latest'],
        'cleanup-server-disk'               => ['POST', '/servers/%s/health/diskcleaner'],

        // web application
        'create-web-app'                            => ['POST', '/servers/%s/webapps/custom'],
        'get-web-apps'                              => ['GET', '/servers/%s/webapps'],
        'get-web-app-details'                       => ['GET', '/servers/%s/webapps/%s'],
        'set-default-web-app'                       => ['POST', '/servers/%s/webapps/%s/default'],
        'remove-default-web-app'                    => ['DELETE', '/servers/%s/webapps/%s/default'],
        'create-web-app-alias'                      => ['POST', '/servers/%s/webapps/%s/alias'],
        'rebuild-web-app'                           => ['POST', '/servers/%s/webapps/%s/rebuild'],
        'clone-git-repo-web-app'                    => ['POST', '/servers/%s/webapps/%s/git'],
        'get-git-repo-web-app-details'              => ['GET', '/servers/%s/webapps/%s/git'],
        'change-git-repo-web-app-branch'            => ['PATCH', '/servers/%s/webapps/%s/git/%s/branch'],
        'customize-git-deploy-script'               => ['PATCH', '/servers/%s/webapps/%s/git/%s/script'],
        'force-deployment-using-deployment-script'  => ['PUT', '/servers/%s/webapps/%s/git/{gitId}/script'],
        'delete-git-repo-web-app'                   => ['DELETE', '/servers/%s/webapps/%s/git/%s'],
        'install-php-script-to-web-app'             => ['POST', '/servers/%s/webapps/%s/installer'],
        'get-web-app-php-script'                    => ['GET', '/servers/%s/webapps/%s/installer'],
        'remove-php-script'                         => ['DELETE', '/servers/%s/webapps/%s/installer/%s'],
        'add-domain-name'                           => ['POST', '/servers/%s/webapps/%s/domains'],
        'get-domain-names'                          => ['GET', '/servers/%s/webapps/%s/domains'],
        'get-domain-name-details'                   => ['GET', '/servers/%s/webapps/%s/domains/%s'],
        'delete-domain-name'                        => ['DELETE', '/servers/%s/webapps/%s/domains/%s'],
        'install-basic-ssl'                         => ['POST', '/servers/%s/webapps/%s/ssl'],
        'get-basic-ssl-config'                      => ['GET', '/servers/%s/webapps/%s/ssl'],
        'update-basic-ssl-config'                   => ['PATCH', '/servers/%s/webapps/%s/ssl/%s'],
        'redeploy-basic-ssl'                        => ['PUT', '/servers/%s/webapps/%s/ssl/%s'],
        'delete-basic-ssl-config'                   => ['DELETE', '/servers/%s/webapps/%s/ssl/%s'],
        'get-advanced-ssl-status'                   => ['GET', '/servers/%s/webapps/%s/ssl/advanced'],
        'switch-advanced-ssl-status'                => ['POST', '/servers/%s/webapps/%s/ssl/advanced'],
        'install-advanced-ssl'                      => ['POST', '/servers/%s/webapps/%s/domains/%s/ssl'],
        'get-advanced-ssl-config'                   => ['GET', '/servers/%s/webapps/%s/domains/%s/ssl'],
        'update-advanced-ssl-config'                => ['PATCH', '/servers/%s/webapps/%s/domains/%s/ssl/%s'],
        'redeploy-advanced-ssl'                     => ['PUT', '/servers/%s/webapps/%s/domains/%s/ssl/%s'],
        'delete-advanced-ssl-config'                => ['DELETE', '/servers/%s/webapps/%s/domains/%s/ssl/%s'],
        'get-web-app-settings'                      => ['GET', '/servers/%s/webapps/%s/settings'],
        'update-web-app-php-version'                => ['PATCH', '/servers/%s/webapps/%s/settings/php'],
        'update-web-app-fpm-nginx-settings'         => ['PATCH', '/servers/%s/webapps/%s/settings/fpmnginx'],
        'get-web-app-activity-log'                  => ['GET', '/servers/%s/webapps/%s/log'],
        'delete-web-app'                            => ['DELETE', '/servers/%s/webapps/%s'],
        
        // database
        'create-database'                   => ['POST', '/servers/%s/databases'],
        'get-databases'                     => ['GET', '/servers/%s/databases'],
        'get-database-details'              => ['GET', '/servers/%s/databases/%s'],
        'delete-database'                   => ['DELETE', '/servers/%s/databases/%s'],
        'create-database-user'              => ['POST', '/servers/%s/databaseusers'],
        'get-database-users'                => ['GET', '/servers/%s/databaseusers'],
        'get-database-user-details'         => ['GET', '/servers/%s/databaseusers/%s'],
        'update-database-user-password'     => ['PATCH', '/servers/%s/databaseusers/%s'],
        'delete-database-user'              => ['DELETE', '/servers/%s/databaseusers/%s'],
        'attach-database-user-to-database'  => ['POST', '/servers/%s/databases/%s/grant'],
        'get-database-granted-users'        => ['GET', '/servers/%s/databases/%s/grant'],
        'revoke-user-from-database'         => ['DELETE', '/servers/%s/databases/%s/grant'],

        // system user
        'create-system-user'            => ['POST', '/servers/%s/users'],
        'get-system-users'              => ['GET', '/servers/%s/users'],
        'get-system-user-details'       => ['GET', '/servers/%s/users/%s'],
        'update-system-user-password'   => ['PATCH', '/servers/%s/users/%s/password'],
        'generate-git-deployment-key'   => ['PATCH', '/servers/%s/users/%s/deploymentkey'],
        'delete-system-user'            => ['DELETE', '/servers/%s/users/%s'],

        // ssh key
        'add-ssh-key'           => ['POST', '/servers/%s/sshcredentials'],
        'get-ssh-keys'          => ['GET', '/servers/%s/sshcredentials'],
        'get-ssh-key-details'   => ['GET', '/servers/%s/sshcredentials/%s'],
        'delete-ssh-key'        => ['DELETE', '/servers/%s/sshcredentials/%s'],

        // cron job
        'create-cron-job'       => ['POST', '/servers/%s/cronjobs'],
        'get-cron-jobs'         => ['GET', '/servers/%s/cronjobs'],
        'get-cron-job-details'  => ['GET', '/servers/%s/cronjobs/%s'],
        'rebuild-cron-jobs'     => ['POST', '/servers/%s/cronjobs/rebuild'],
        'delete-cron-job'       => ['DELETE', '/servers/%s/cronjobs/%s'],

        // supervisor
        'get-available-supervisor-binary'   => ['GET', '/servers/%s/supervisors/binaries'],
        'create-supervisor-job'             => ['POST', '/servers/%s/supervisors'],
        'get-supervisor-jobs'               => ['GET', '/servers/%s/supervisors'],
        'get-supervisor-job-status'         => ['GET', '/servers/%s/supervisors/status'],
        'rebuild-supervisor-jobs'           => ['POST', '/servers/%s/supervisors/rebuild'],
        'get-supervisor-job-config'         => ['GET', '/servers/%s/supervisors/%s'],
        'reload-supervisor-job'             => ['POST', '/servers/%s/supervisors/%s/reload'],
        'delete-supervisor-job'             => ['DELETE', '/servers/%s/supervisors/%s'],

        // services
        'get-services'      => ['GET', '/servers/%s/services'],
        'trigger-service'   => ['PATCH', '/servers/%s/services'],

        // security
        'create-firewall-rule'                  => ['POST', '/servers/%s/security/firewalls'],
        'get-firewall-rules'                    => ['GET', '/servers/%s/security/firewalls'],
        'get-firewall-rule-config'              => ['GET', '/servers/%s/security/firewalls/%s'],
        'deploy-firewall-rules-to-server'       => ['PUT', '/servers/%s/security/firewalls'],
        'delete-firewall-rule'                  => ['DELETE', '/servers/%s/security/firewalls/%s'],
        'get-fail2ban-blocked-ip-addresses'     => ['GET', '/servers/%s/security/fail2ban/blockedip'],
        'delete-fail2ban-blocked-ip-address'    => ['DELETE', '/servers/%s/security/fail2ban/blockedip'],
        
        // log
        'get-server-logs'   => ['GET', '/servers/%s/logs']
    ]
];
