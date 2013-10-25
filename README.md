IRC dance bot for Maplin Robot Arm
===================================

An IRC bot that controls a Maplin Robotic Arm
(http://www.maplin.co.uk/robotic-arm-kit-with-usb-pc-interface-266257)

Usage
-----

`
./robopi [-secure] <host[:port]>
`

On IRC, address the bot with 'robopi: <command>'.

Commands:

`
move <body part> <direction> <duration>
	Add a move to the current dance or execute a move
	if not currently learning a dance

teach <dance name>
	Start teaching the bot a dance

done
	Finish teaching the dance

dance <dance name>
	Perform a saved dance

forget <dance name>
	Forget a saved dance
`

Why?
----

Erm ... I had a robot arm, a Raspberry Pi and wanted to learn a little about Go and IRC :)
