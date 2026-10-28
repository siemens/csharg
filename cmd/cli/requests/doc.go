/*
Package requests provides the global “--request-timeout” CLI flag for limiting how
long to wait for a single capture service request.

The flag is automatically added to the root command as a persistent flag. Client
providers get the timeout for the command being executed by calling
[GetTimeout]:

	timeout := requests.GetTimeout(cmd)

A zero timeout means to never time out requests.
*/
package requests
