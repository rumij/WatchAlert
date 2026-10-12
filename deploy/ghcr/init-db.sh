#!/bin/sh
set -eu
for file in notice_template_examples.sql rule_template_groups.sql rule_templates.sql tenants.sql tenants_linked_users.sql; do
  mysql --protocol=TCP -h w8t-mysql -u watchalert --default-character-set=utf8mb4 -D watchalert < "/sql/$file"
done
echo 'WatchAlert seed data imported successfully.'
