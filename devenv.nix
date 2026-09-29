{ pkgs, ... }:

{
  # pdftotext for Elternbrief and message attachments.
  packages = [ pkgs.poppler-utils ];
}
