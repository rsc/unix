This is a Research Unix Sixth Edition (v6) kernel written in Go
and using rsc.io/unix/pdp11 to run user-mode code.

The kernel follows the structure of the actual v6 kernel.
The core system call implementations are direct translations
from the original C to Go. Other files are new, to provide
the abstractions the core files need without having to deal
with the messy hardware details of an actual PDP11 like
in simh.

For simplicity, the file system is maintained entirely in memory,
as an array of inodes with a plain []byte to hold the data.
There is no disk, so no disk blocks, no disk block locking,
no sleeping during file system operations.
The data structures are set up at startup from the content
of disk.txtar, which is embedded in the package.
The mkdisk script builds disk.txtar from the original v6 disks
in ../v6 with the files in local.txtar layered on top.
Running mkdisk also leaves a tree behind in _fs, so you may
want to run mkdisk just to inspect the files in _fs.

A group of tty devices is simulated to allow programs like login to run.
