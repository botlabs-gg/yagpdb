    {
        "author": "jonas747",
        "date": "4th Oct 2016",
        "title": "Version 0.15"
    }

 - Custom Commands: Added user as template data
 - Web: Improved form validation most places
 - Bot: Squashed the persistent no message timestamp bug once and for all...
 - Bot: Join/Leave messages sent in the same second will now be merged togheter (to handle things like purges of thousands of members better)
 - Bot: Removed the refreshstreaming command since it's no longer needed as everything will auto update
 - Bot: Improve member loading on startup (now uses a gateway request instead of rest request)
 - Backend: Redid all logging