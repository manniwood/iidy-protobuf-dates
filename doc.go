/*
Package iidy is a REST-like checklist or "attempt list"
with a PostgreSQL backend.

Sample interaction:

	$ curl localhost:8080/iidy/v1/lists/downloads/a.txt
	Not found.

	$ curl -X POST localhost:8080/iidy/v1/lists/downloads/a.txt
	ADDED 1

	$ curl localhost:8080/iidy/v1/lists/downloads/a.txt
	0

	$ curl -X POST localhost:8080/iidy/v1/increment/lists/downloads/a.txt
	INCREMENTED 1

	$ curl localhost:8080/iidy/v1/lists/downloads/a.txt
	1

	$ curl -X DELETE localhost:8080/iidy/v1/lists/downloads/a.txt
	DELETED 1

	$ curl localhost:8080/iidy/v1/lists/downloads/a.txt
	Not found.

	$ curl -X POST localhost:8080/iidy/v1/batch/lists/downloads -d '
	b.txt
	c.txt
	d.txt
	e.txt
	f.txt
	g.txt
	h.txt
	i.txt
	'
	ADDED 8

	$ curl localhost:8080/iidy/v1/batch/lists/downloads?count=2
	b.txt 0
	c.txt 0

	$ curl "localhost:8080/iidy/v1/batch/lists/downloads?count=2&after_id=c.txt"
	d.txt 0
	e.txt 0

	$ curl "localhost:8080/iidy/v1/batch/lists/downloads?count=4&after_id=e.txt"
	f.txt 0
	g.txt 0
	h.txt 0
	i.txt 0

	$ curl localhost:8080/iidy/v1/increment/batch/lists/downloads
	b.txt
	c.txt
	d.txt
	e.txt
	'
	INCREMENTED 4

	$ curl localhost:8080/iidy/v1/batch/lists/downloads?count=100
	b.txt 1
	c.txt 1
	d.txt 1
	e.txt 1
	f.txt 0
	g.txt 0
	h.txt 0
	i.txt 0

	$ curl -X DELETE localhost:8080/iidy/v1/batch/lists/downloads -d '
	d.txt
	e.txt
	f.txt
	g.txt
	'
	DELETED 4

	$ curl localhost:8080/iidy/v1/batch/lists/downloads?count=100
	b.txt 1
	c.txt 1
	h.txt 0
	i.txt 0
*/
package iidy
