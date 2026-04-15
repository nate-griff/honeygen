- There's no easy way to view error logs for web gui errors. For example, I can't get the generation tab to load, it just responds `## Unable to load the admin UI    Cannot read properties of null (reading 'some')` and the same happens for the other tabs of File Browser and Event Log. This is on a fresh startup with no files created/generated
- World models is a bit cumbersome to fill out. We already have an attached LLM, maybe have another box that you can request a world model be generated based on a few sentences, then that shows up as a new model with everything filled out, then the user can tweak as needed. Basically turns natural language into a world model.
- Have Dashboard live update with recent events/assets
- Have a way to update LLM config on the web UI since they can be tricky to work with sometimes
- Add comments to .env.example with basic instructions (docker can read .env files with comments)
- Add file viewing/preview capability into the file browser
- The readme says to do this: `Invoke-RestMethod -Method Post http://localhost:8080/api/provider/test` but that rejects because an API key is required. Same goes for the rest of the stuff in the readme. Doesn't seem to be an LLM API key error since I set the api key in llama.cpp and did the same api key in env and got this: 
```PS C:\Users\natet> Invoke-RestMethod -Method Post http://127.0.0.1:8080/api/provider/test
Invoke-RestMethod : {"error":{"code":"provider_invalid","message":"provider API key is required"}}
At line:1 char:1
+ Invoke-RestMethod -Method Post http://127.0.0.1:8080/api/provider/tes ...
+ ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : InvalidOperation: (System.Net.HttpWebRequest:HttpWebRequest) [Invoke-RestMethod], WebException
    + FullyQualifiedErrorId : WebCmdletWebResponseException,Microsoft.PowerShell.Commands.InvokeRestMethodCommand
```
- Regarding the model, I'm running llama.cpp on 0.0.0.0 port 8033, so I did http://host.docker.internal:8033/v1 and it still shows Provider as unconfigured. 
- Another error:
```
Invoke-RestMethod `
>>   -Method Post `
>>   -Uri http://localhost:8080/api/generation/run `
>>   -ContentType 'application/json' `
>>   -Body '{"world_model_id":"northbridge-financial"}'
Invoke-RestMethod : {"id":"job_72f030faf747304c797b","world_model_id":"northbridge-financial","status":"failed","started_at":"2026-04-13T16:35:28Z","comple
ted_at":"2026-04-13T16:35:28Z","error_message":"provider API key is required","summary":{"manifest_count":21,"asset_count":0,"categories":["desktop-referen
ce","employee-roster-excerpt","faq-help-page","internal-memo","intranet-about-page","meeting-notes","policy-document","project-summary","public-about-page"
,"vendor-contact-csv"],"logs":[{"time":"2026-04-13T16:35:28.33068915Z","level":"info","message":"generation job
created"},{"time":"2026-04-13T16:35:28.330736131Z","level":"info","message":"planned 21
assets"},{"time":"2026-04-13T16:35:28.333244011Z","level":"info","message":"generating
asset","path":"public/about.html","category":"public-about-page"},{"time":"2026-04-13T16:35:28.335720511Z","level":"error","message":"provider generation
failed: provider API key is
required","path":"public/about.html","category":"public-about-page"}]},"created_at":"2026-04-13T16:35:28Z","updated_at":"2026-04-13T16:35:28Z"}
At line:1 char:1
+ Invoke-RestMethod `
+ ~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : InvalidOperation: (System.Net.HttpWebRequest:HttpWebRequest) [Invoke-RestMethod], WebException
    + FullyQualifiedErrorId : WebCmdletWebResponseException,Microsoft.PowerShell.Commands.InvokeRestMethodCommand
```
- After a full docker container reset and volume deletion, it looks like the website can now interact with the model. Maybe it was an issue with the .env not being read on restarts or builds but the http://host.docker.internal:8033/v1 works now.
- Ok turns out that when you make a new world model or edit one, you then get the "## Unable to load the admin UI    Cannot read properties of null (reading 'some')" error
- Weird workaround which might help pinpoint the error, when I do `Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/generation/run -ContentType 'application/json' -Body '{"world_model_id":"northbridge-financial"}'` it then fixes the broken admin UI pages and allows me to go to Generation tab (even though the job fails). The job also fails when I run generation from the web gui with "Bad Gateway" popping up and the error is: `- **error** · provider generation failed: provider request failed · public/about.html`
- Event Log still fails to load with no events, but if I load up the canary site (localhost:8081) and generate a log, then it shows up. Also, it shows it as coming from 172.18.0.1 because of the docker container, but I wonder if we did something like a cloudlfared tunnel if it would give us the actual IPs. 
- The BACKLOG.md was never updated during initial code generation, please update it to reflect the current progress. I think it's done, but the codebase is in need of a full review, so take a look at INITIAL_PROMPT.md and PROJECT_SPEC.md and see how this code was generated and review the work to make sure it actually implemented it to plan and update the BACKLOG.md as you do this review. This is a big task, so make sure to properly plan this one out. 
- For this prompt, you're in planning mode right now, but I want you to plan, then go to autopilot and continue running until all tasks assigned are fixed and completed. Feel free to ask questions during planning, but once you're on autopilot, I plan on walking away to let you complete this. For context, here's the dev env:
```
APP_ENV=development
APP_VERSION=dev
API_PORT=8080
ADMIN_WEB_PORT=4173
DECOY_WEB_PORT=8081
API_HTTP_ADDR=:8080
DECOY_WEB_HTTP_ADDR=:8081
CONFIG_PATH=/app/config/config.json
SQLITE_PATH=/app/storage/sqlite/honeygen.db
STORAGE_ROOT=/app/storage
GENERATED_ASSETS_DIR=/app/storage/generated
INTERNAL_API_BASE_URL=http://api:8080
INTERNAL_EVENT_INGEST_TOKEN=iZOoHHcLNzKdOyQ07Bru7eivTmIRS10a
VITE_API_BASE_URL=
LLM_BASE_URL=http://host.docker.internal:8033/v1
LLM_API_KEY=dev
LLM_MODEL=Gemma-4-E4B-Claude-Abliterated.Q4_K_M.gguf
```
I'll keep the llama.cpp server running on 0.0.0.0:8033 (host.docker.internal:8033 from the containers POV). I've stopped the docker container