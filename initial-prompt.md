Create a new github repo under the jedarden organization named CLASP. Mana stands for Claude Language Agent Super Proxy. 

Clone the repo to this workspace. 

Create a prompt.md file which describes the looped process that is managed by the github autonomous agent skill. Use that skill and workflow to create a github issue referencing CLASP and its development progress. Ensure that clasp can be installed into this environment where its contents are stored into .clasp/ folder in the workspace root. CLASP should also have some form fo self-updating capability, and barring that a way for claude code to swap in a new CLASP binary and then relaunch CLASP. 

CLASP itself is described in research/claudish there are notes as well as an architecture document. 

after the prompt.md is created in the repo folder, push the prompt to github. 

Create a shell script that is meant to be run in a new tmux instance. The shell script should create a while loop into which claude code headless is invoked. Leave 5 seconds between runs of claude code headless in the while loop. Ensure that claude is run with --dangerously-skip-permissions and stream-json enabled. 

Create a separate shell script which receives the piped stream-json output and makes it more human readable. Keep iterating until no more json is shown in the terminal. Copy the shell script in MANA if needed. 

In the outer shell script include some formatting to identify which iteration is running as well as the localtime with timezone at the point the iteration header was generated. 

After the two scripts and prompt are created, create a new tmux session following the naming scheme in start.sh and run the launcher shell script into it. 

The expectation is that every loop, claude code will use gh cli to check if there's any new comments it did not write from which instruction or guidance is taken. 

After that, go through the prompt and based on the investigation within teh repo and workspace, determine the single highest priority task to accomplish. After accomplishing that task, push any code chanages to github, create a new release if warranted--include the binary in the release, and update the github issue. 

The goal is three-fold

1) Get the proxy working and confirm it works with the OpenAI API in a new tmux instance with claude code interactive running. There will be a .env file in the repo folder where a valid openai api key will be available for testing.
2) Improve the speed and reliability of the proxy. 
3) Set up the openrouter headers to list this application as CLASP


