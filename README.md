# roolet

Web-socket server for call remote functions in workers pool,
first worker-server library will use python3 and based on multiprocessing.

Common thinks:

- golang used for this server

- RPC based on custom protocol with JSON data exchange

- for web-socket server used popular project

- collect statistics

Options format example:

	{
		"node": "node1",
		"addr": "127.0.0.1",
		"port": 7551,
		"wsaddr": "127.0.0.1",
		"wsport": 7555,
		"buffersize": 512,
		"workers": 4,
		"countworkertime": true,
		"statistic": true,
		"statisticchecktime": 5,
		"keysize": 32,
		"secret": "688dverxjga0ya87myzssshy8yrsbvgmn5t3qt57yvpkdxyqmnp3qbf8ms0wd99e"
	}

As simple, this is look like this:

![planed architecture](doc/architecture.png?raw=true "how to use")
