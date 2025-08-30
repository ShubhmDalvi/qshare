@echo off
echo Adding QShare firewall rule for all profiles...
netsh advfirewall firewall add rule name="QShare" dir=in action=allow program="%~dp0qshare.exe" profile=any enable=yes
echo Done! You may still need to allow in 3rd-party antivirus firewalls.
pause
