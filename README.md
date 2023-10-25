rsc.io/unix holds programs for running old Unix programs on modern computers.

[pdp11](pdp11/) is a PDP-11 simulator.

[v6unix](v6unix/) is a Research Unix Sixth Edition (V6) simulator. It is a port of the V6 kernel logic to Go, using the PDP11 simulator to run user programs. For the most part the kernel is a faithful simulation of the V6 kernel, but it is written to use in-memory data structures and other simplifying assumptions and doesn't have to worry at all about the specific details of PDP11 disks, terminals, and other hardware. This lets users focus on how Unix programs worked and what is was like to use the system, instead of learning how to configure simulated RK05 disk packs.

[v6run](v6run/) is a command-line interface to v6unix. `go run rsc.io/unix/v6run@latest` will run the simulator. Typing Control-Backslash will exit the simulator.

[v6web](v6web/) is a web browser-based interface to v6unix. To use it, you have to cd into that directory and then run:

	go generate
	go run serve.go

That will serve a web version at http://localhost:8080/.

