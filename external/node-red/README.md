# Node-RED

http://nodered.org

[![Build Status](https://travis-ci.org/node-red/node-red.svg?branch=master)](https://travis-ci.org/node-red/node-red)
[![Coverage Status](https://coveralls.io/repos/node-red/node-red/badge.svg?branch=master)](https://coveralls.io/r/node-red/node-red?branch=master)

Low-code programming for event-driven applications.

![Node-RED: Low-code programming for event-driven applications](http://nodered.org/images/node-red-screenshot.png)

## Quick Start

1. Get Node-Red 2.2.2 from github

        Install the node-red dependencies

        npm install

2. Build the code

        npm run build

3. Run

        npm start - confirm node-red is running

4. Overwrite Settings and theme

Go to hedge/external/node-red/theme and copy files to node-red deploy folder
Go to /hedge/external/node-red/settings.js

copy editorTheme sections page and header objects and paste them in to node-red deploy folders's settings.js file
edit the file locations for favicon, css, and scripts to reference where you copied the theme folder.

now overwrite the node-red settings file if you are unsure of where this is goto your console and when you start node red it will state where the settings file is.

update file locations to the new aboslute file locations of the theme directory in the node-red deploy folder

save settings.js and now copy it to node-red deploy folder/settings.js


## Contributing

Before raising a pull-request, please read our
[contributing guide](https://github.com/node-red/node-red/blob/master/CONTRIBUTING.md).

This project adheres to the [Contributor Covenant 1.4](http://contributor-covenant.org/version/1/4/).
 By participating, you are expected to uphold this code. Please report unacceptable
 behavior to any of the project's core team at team@nodered.org.

## Authors

Node-RED is a project of the [OpenJS Foundation](http://openjsf.org).

It is maintained by:

 * Nick O'Leary [@knolleary](http://twitter.com/knolleary)
 * Dave Conway-Jones [@ceejay](http://twitter.com/ceejay)
 * And many others...


## Copyright and license

Copyright OpenJS Foundation and other contributors, https://openjsf.org under [the Apache 2.0 license](LICENSE).
