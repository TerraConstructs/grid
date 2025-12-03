Output ./examples/demo/demo.gif
Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 800
Set Framerate 24
Set Padding 5

Hide
Type `export PATH="$PWD/bin/:$PATH"` Enter
Ctrl+l
Type `Set up database`
Ctrl+a
Show
Sleep 2s
Delete 15

Type `make db-reset` Enter
Sleep 2s

Type `gridapi db init` Enter
Sleep 3s

Type `gridapi db migrate` Enter
Sleep 2s

Hide
Type `Start API`
Ctrl+a
Show
Sleep 2s
Delete 9

Type `gridapi serve &`
Sleep 0.5s
Enter
Sleep 1s

Type `cd examples/terraform/network`
Sleep 0.5s
Enter

Hide
Type `Set up the network terraform state`
Ctrl+a
Show
Sleep 2s
Delete 34

Type `gridctl state create network`
Sleep 0.5s
Enter
Sleep 2s

Type `gridctl state get`
Sleep 0.5s
Enter
Sleep 3s
Type `cat .grid` Enter
Sleep 2s

Type `gridctl state init`
Sleep 0.5s
Enter
Sleep 2s
Type `cat backend.tf` Enter
Sleep 2s
Type `tofu init`
Sleep 0.5s
Enter
Sleep 3s
Type `tofu apply -auto-approve`
Sleep 0.5s
Enter
Sleep 2s
Type `gridctl state get`
Sleep 0.5s
Enter
Sleep 2s

Hide
Type `Set up the cluster terraform state`
Ctrl+a
Show
Sleep 2s
Delete 34

Type `cd ../cluster`
Sleep 0.5s
Enter
Type `gridctl state create cluster` 
Sleep 0.5s
Enter
Sleep 2s

Type `gridctl state get`
Sleep 0.5s
Enter
Sleep 2s

Type `gridctl state init`
Sleep 0.5s
Enter
Sleep 2s

Type `cat null_resources.tf` Enter
Sleep 5s
Type `gridctl deps add --from network`
Sleep 0.5s
Enter
Sleep 2s
Enter
Down
Enter
Sleep 1s
Tab
Sleep 2s
Type `gridctl deps sync`
Sleep 0.5s
Enter
Sleep 2s
Type `cat grid_dependencies.tf` Enter
Sleep 5s

Hide
Type `Review State and Dependencies status`
Ctrl+a
Show
Sleep 2s
Delete 36

Type `gridctl state get`
Sleep 0.5s
Enter
Sleep 3s

Type `gridctl state list`
Sleep 0.5s
Enter
Sleep 3s

Type `gridctl deps list`
Sleep 0.5s
Enter
Sleep 3s

Type `gridctl deps topo --direction upstream`
Sleep 0.5s
Enter
Sleep 3s


Hide
Type `Consume Dependencies ...`
Ctrl+a
Show
Sleep 2s
Delete 24

Type `tofu init`
Sleep 0.5s
Enter
Type `tofu apply -auto-approve`
Sleep 0.5s
Enter
Sleep 2s


Hide
Type `Review State and Dependencies status`
Ctrl+a
Show
Sleep 2s
Delete 36

Type `gridctl state get`
Sleep 0.5s
Enter
Sleep 3s

Type `gridctl state list`
Sleep 0.5s
Enter
Sleep 3s

Type `gridctl deps list`
Sleep 0.5s
Enter
Sleep 3s

Hide
Type `Trigger change on network`
Ctrl+a
Show
Sleep 2s
Delete 25

Type `cd ../network`
Sleep 0.5s
Enter
Type `tofu apply`
Sleep 0.5s
Enter
Sleep 2s
Type `yes`
Sleep 0.5s
Enter

Type `gridctl deps list --state cluster`
Sleep 0.5s
Enter
Sleep 3s


Hide
Type `Re-apply cluster`
Ctrl+a
Show
Sleep 2s
Delete 16

Type `cd ../cluster`
Sleep 0.5s
Enter
Type `tofu apply`
Sleep 0.5s
Enter
Sleep 1s
Type `yes`
Sleep 0.5s
Enter

Type `gridctl deps list`
Sleep 0.5s
Enter
Sleep 3s

# Clean up

Type `fg` Enter
Ctrl+c Sleep 5s
