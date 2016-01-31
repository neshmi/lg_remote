# LG IP Remote

This [Go](http://golang.org) based remote is a simple IP based remote for controlling multiple LG Smart TVs, written to ease work with CAVE environments (see [CalVR](https://github.com/calvr)).

I wrote this in Go as an exercise to teach myself Go, so it is probably poorly written and could use some serious revisions.

The remote will look for a JSON file, if you set LG_REMOTE_PATH and LG_REMOTE_CONFIG_FILE it will open that file, otherwise it defaults to a file called tv_config.json which is in your current directory.

# Sources
This was inspired by:
- [https://github.com/ubaransel/lgcommander](https://github.com/ubaransel/lgcommander)
- [https://github.com/dreamcat4/lgremote](https://github.com/dreamcat4/lgremote)
