iban-service
==============

Implements a basic REST Web-service for validating IBAN account numbers. Uses the logic from http://github.com/fourcube/goiban.

This is a down stripped version of http://github.com/fourcube/goiban-service.

# Running the service

## Download a binary package:

A list of all releases is available [here](https://github.com/malaupa/iban-service/releases).

```bash
# Make sure to choose the correct operating system and architecture
$ curl -Lo iban-service.tar.gz "https://github.com/malaupa/iban-service/releases/download/v1.0.0/iban-service-1.0.0-linux-386.tar.gz"
$ tar -xzf iban-service.tar.gz
$ cd iban-service
# Launch the service listening on port 8080, using the bank data from ./data and serving
$ ./iban-service -dataPath ./data -port 8080
```
To test instance:

```
$ curl localhost:8080/validate/DE89370400440532013000
```

You will see something like:

```json
{
  "valid": true,
  "messages": [],
  "iban": "DE89370400440532013000",
  "bankData": {
    "bankCode": "",
    "name": ""
  },
  "checkResults": {}
}
```

Providing new data
-------

Data files provided by some european bank institutes (e.g Austria and Germany) is sometimes provided as a ISO-8859-1 encoded file.
It should be converted to UTF-8 before being committed to the repository.

This is possible using `iconv`.

```bash
# Austria
$ iconv -f iso-8859-1 -t utf8 at_original.csv > at.csv
# Germany
$ iconv -f iso-8859-1 -t utf8 bundesbank_original.txt > bundesbank.txt
```