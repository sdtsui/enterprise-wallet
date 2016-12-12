<?php

$pageTitle = 'New Factoid Address';
$mainClass = 'send-factoids';
$activeNav = 2;

ob_start(); ?>

<div class="row">
                        <div class="columns">
                            <a href="/address-book.php" class="button close-button" data-close aria-label="Close reveal"><span aria-hidden="true">&times;</span></a>
                            <h1>New Factoid Address</h1>
                            <form>
                                <div class="row">
                                    <div class="small-12 medium-8 large-6 columns">
                                        <label>Generate address from:</label>
                                        <select id="generate-source" name="generate-source">
                                            <option value="random">Random new address</option>
                                            <option value="private-key">Private key</option>
                                            <option value="twelve-words">Twelve words</option>
                                        </select>
                                    </div>
                                </div>
                                <div class="row">
                                    <div class="small-12 medium-7 large-8 columns">
                                        <label>Public key:</label>
                                        <input type="text" name="public-key">
                                    </div>
                                    <div class="small-12 medium-5 large-4 columns">
                                        <label>Nickname:</label>
                                        <input type="text" name="alias" placeholder="Alias of address">
                                    </div>
                                </div>
                                <div class="row">
                                    <div class="medium-8 hide-for-small-only columns">

                                    </div>
                                    <div class="medium-4 small-12 columns">
                                        <a href="#" class="button expanded">Add to Address Book</a>
                                    </div>
                                </div>
                            </form>
                        </div>
                    </div>

<?php

$mainContent = ob_get_contents();
ob_end_clean();

include 'template.php';

?>