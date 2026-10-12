# vlinedemo

Demo plugin for the veil virtual-lines API.

* `vlinebox [text]`: draw a box of virtual lines below the cursor line. The
  border uses the `comment` colorscheme group and the text uses `special`.
* `vlinehide`: hide the cursor line itself. It reappears whenever the cursor
  is on it.
* `vlineclear`: remove everything this plugin added.

Virtual lines are display-only: they are never saved and can't hold the
cursor.
