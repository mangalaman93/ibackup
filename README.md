# ibackup
Simple influxdb incremental backup program to download data in small chunks to avoid crashes

# How to use
```
go install
ibackup --database=db --host=0.0.0.0:8086 -o backup --password=pass --username=user
```

# TODO
* fix timestamp, use influxdb server timestamp instead
* download data in steps of time, don't download the whole series
* restore mechanism
* stream the output of curl, show progress bar
* curl?

# Cautions
* Make sure that no new data is stored with older timestamp after backup is created
* Incremental backup is stored based on current timestamp of local host
