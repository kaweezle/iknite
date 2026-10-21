## To do

- [ ] Add logger to store
- [ ] Move download progress to utils
- [ ] Change filenames to digests in order to easily detect duplicates and avoid
      collisions
- [ ] Have the image service created at the image command level and passed down
      to the subcommands, instead of creating a new instance in subcommands.
- [ ] On the image info command, by default display a tree information structure
      and provide a `--json` flag to display the full information in JSON
      format.
- [ ] On the list command, remove the highlighting of the first line.
