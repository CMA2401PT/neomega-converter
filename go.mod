module neomega-converter

go 1.25.5

require (
	celler v0.0.0
	neomega-blocks v0.0.0
	neomega-nbt v0.0.0
)

require (
	github.com/df-mc/goleveldb v1.1.9 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	neomega-packfs v0.0.0 // indirect
)

replace neomega-blocks => ../neomega-blocks

replace neomega-nbt => ../neomega-nbt

replace celler => ../neomega-celler

replace neomega-packfs => ../neomega-packfs
