# yabs-poc

proof of concept for [y]et [a]nother [b]uild [s]ystem

## Goals
* Composable, write Go for targets, rules
* Distribute `yabs` as a binary, not as a module
* Build ~15 projects efficiently
* Replace `make`
* Run in CI and locally

## Non-goals
* Build millions+ LoC