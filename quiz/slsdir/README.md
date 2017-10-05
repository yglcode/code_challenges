Please only do this exercise when explicitly asked. This is for select candidates only. 

#Exercise 1

The `slsdir` contains `primary` which contains a directory with an ID of a machine, a random number.

Under it, there are a set of files, each of which is named with a UNIX style timestamp as a prefix.
The first line of each file contains column names, each of which is separated by a tab character,
followed by a line with redundant ID of the machine. The rest of the lines are metric data. Each line
has values for each of the columns. The first item is the UNIX timestamp, in seconds, since UNIX epoch.
Some of the files have duplicate data which has the timestamp value as some other lines in other files.

Step 1, write a program in Go to concatenate all files into one file. While doing so, remove any duplicate timestamp'ed data lines.
Lines of metric data in the final file should be ordered by timestamps, which is the first column data item.
Step 2, enhance the Go program to reduce the lines of metric data from the file in step 1. 
The result of the reduction should be processed values of each columns over time.  
Processing of values should result in summary of all values in the following representations, per column, except
the timestamp:

```
	period			the range of time period, starting timestamp and stopping timestamp
	count 			number of values
	min 			minimum of values
	max 			maximum of values
	mean 			mean of values
	std-dev 		standard deviation
	50-precentile 		
	75-percentile 
	95-percentile 
	99-percentile 
	999-percentile
```

You may use this package if you want:  github.com/rcrowley/go-metrics
