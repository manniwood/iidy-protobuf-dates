#!/usr/bin/env bash
set -eux
set -o pipefail

sudo apt-get remove -y --purge postgresql libpq-dev libpq5 postgresql-client-common postgresql-common
sudo rm -rf /var/lib/postgresql
sudo apt-get install -y postgresql-common
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
sudo apt-get update -qq
sudo apt-get -y -o Dpkg::Options::=--force-confdef -o Dpkg::Options::="--force-confnew" install postgresql-$PGVERSION postgresql-server-dev-$PGVERSION postgresql-contrib-$PGVERSION
sudo chmod 777 /etc/postgresql/$PGVERSION/main/pg_hba.conf
echo "local     all         postgres                          trust"    >  /etc/postgresql/$PGVERSION/main/pg_hba.conf
echo "local     all         all                               trust"    >> /etc/postgresql/$PGVERSION/main/pg_hba.conf
echo "host      all         postgres     127.0.0.1/32         trust"    >> /etc/postgresql/$PGVERSION/main/pg_hba.conf
echo "host      all         postgres     ::/0                 trust"    >> /etc/postgresql/$PGVERSION/main/pg_hba.conf
sudo chmod 777 /etc/postgresql/$PGVERSION/main/postgresql.conf
sudo /etc/init.d/postgresql restart

