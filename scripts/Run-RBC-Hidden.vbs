Option Explicit
Dim shell, encoded, pattern, command, result
If WScript.Arguments.Count <> 1 Then WScript.Quit 2
encoded = WScript.Arguments(0)
Set pattern = New RegExp
pattern.Pattern = "^[A-Za-z0-9+/=]+$"
If Not pattern.Test(encoded) Then WScript.Quit 2
Set shell = CreateObject("WScript.Shell")
command = Chr(34) & shell.ExpandEnvironmentStrings("%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe") & Chr(34)
command = command & " -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -EncodedCommand " & encoded
' WScript is a GUI host: create the child hidden before PowerShell initializes.
' Wait and propagate its result so Task Scheduler still reports real failures.
result = shell.Run(command, 0, True)
WScript.Quit result
