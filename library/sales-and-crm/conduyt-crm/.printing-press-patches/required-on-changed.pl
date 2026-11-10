#!/usr/bin/perl
# PATCH(reprint-637-required-on-changed): post-print step, run after every reprint (Codex reprint637-r3).
#
# Printing Press 4.28.0 serializes a scalar body flag only when its value is non-zero (`if bodyX != "" {`,
# `!= 0 {`, `!= 0.0 {`, `if bodyX {`), so a REQUIRED flag passed with its zero value ("" / 0 / 0.0 / false) is
# silently left out of the body. The generator's template lives in the press (upstream, pinned at 4.28.0), so the
# rule is applied here, once, to every generated command. A body flag is REQUIRED when the file checks it with
# `required flag "%s" not set", "<flag>"` (the generator's own list) or when tools-manifest.json marks the body
# param of the command's operation required (a required param with a spec default, which gets no check). Its
# serializer guard becomes `if cmd.Flags().Changed("<flag>") {`; that includes the whole-body `--body-json`
# (flagBodyJSON) of anyOf/oneOf bodies, whose empty value now fails to parse locally instead of sending {}.
# Optional flags and query flags keep the generator's guard. Hand-shaped required sets (emails create-template
# --body/--channel/--name) carry the same marker by hand. Idempotent.
# Pinned by TestEveryRequiredBodyFlagSerializesItsZeroValue (internal/cli/required_on_changed_test.go).
#
# Usage (repo root): perl .printing-press-patches/required-on-changed.pl internal/cli/*.go
use strict;
use warnings;
use JSON::PP;

my %manifestRequired;    # "METHOD path" => { flag => 1 }
{
    open(my $m, '<', 'tools-manifest.json') or die "tools-manifest.json: $! (run from the repo root)";
    my $doc = decode_json(do { local $/; <$m> });
    close $m;
    for my $tool (@{ $doc->{tools} }) {
        for my $p (@{ $tool->{params} || [] }) {
            next unless $p->{required} && ($p->{location} // '') eq 'body';
            (my $flag = $p->{name}) =~ s/_/-/g;
            $flag =~ s/(?<=.)([A-Z])/-\l$1/g;
            $manifestRequired{"$tool->{method} $tool->{path}"}{ lc $flag } = 1;
        }
    }
}

my ($files, $guards) = (0, 0);
for my $file (@ARGV) {
    next if $file =~ /_test\.go$/;
    open(my $in, '<', $file) or die "$file: $!";
    my $src = do { local $/; <$in> };
    close $in;
    my %required = map { $_ => 1 } ($src =~ /required flag \\"%s\\" not set", "([a-z0-9-]+)"\)/g);
    while ($src =~ /"pp:method": "([A-Z]+)", "pp:path": "([^"]+)"/g) {
        my $req = $manifestRequired{"$1 $2"} or next;
        $required{$_} = 1 for keys %$req;
    }
    next unless %required;
    my %var;
    while ($src =~ /Flags\(\)\.(?:String|Int|Int64|Float64|Bool)Var\(&(\w+), "([a-z0-9-]+)"/g) {
        my ($v, $f) = ($1, $2);
        $var{$v} = $f if $required{$f} && ($v =~ /^body[A-Z]/ || $v eq "flagBodyJSON"); # body fields (and the whole-body --body-json) only: a required QUERY flag keeps the generator guard
    }
    my $n = 0;
    for my $v (keys %var) {
        my $flag = $var{$v};
        my $mark = qq|if cmd.Flags().Changed("$flag") { // PATCH(reprint-637-required-on-changed)|;
        $n += $src =~ s/^(\t+)if \Q$v\E(?: != ""| != 0| != 0\.0)? \{$/$1$mark/mg;
    }
    # r2 per-flag fixes (reprint-637-required-zero-values) are the same rule: one marker.
    $n += $src =~ s{\{ // PATCH\(reprint-637-required-zero-values\)[^\n]*$}{\{ // PATCH(reprint-637-required-on-changed)}mg;
    next unless $n;
    open(my $out, '>', $file) or die "$file: $!";
    print $out $src;
    close $out;
    $files++;
    $guards += $n;
}
print "required-on-changed: $guards guards in $files files\n";
